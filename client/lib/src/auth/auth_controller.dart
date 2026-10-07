import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/oidc_platform.dart';
import 'package:meowgram_client/src/auth/oidc_service.dart';
import 'package:meowgram_client/src/auth/pkce_helper.dart';
import 'package:meowgram_client/src/auth/token_storage.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:url_launcher/url_launcher.dart';

/// Opens the authorization URL. Injectable for tests.
typedef UrlLauncher = Future<bool> Function(Uri uri, {required bool sameTab});

Future<bool> _defaultLauncher(Uri uri, {required bool sameTab}) => launchUrl(
      uri,
      mode: LaunchMode.externalApplication,
      webOnlyWindowName: sameTab ? '_self' : null,
    );

/// Central authentication state controller managing the OIDC PKCE lifecycle,
/// active token storage, token auto-refresh timers, and user profile state.
///
/// Offline-first rules:
/// - [initialize] only reads secure storage; it never waits on the network, so the
///   first frame (and the cached timeline) is never blocked.
/// - A stored session stays signed in while the access token is expired; it is
///   refreshed in the background. Only a definitive rejection by Authelia
///   ([OidcTokenRejectedException]) ends the session. Network errors keep it and
///   retry later.
/// - Refreshes are single-flight, so a timer and an app resume can't both spend
///   the same (rotating) refresh token.
class AuthController extends ChangeNotifier {
  final OidcService _oidcService;
  final OidcPlatformHelper _platformHelper;
  final TokenStorage _tokenStorage;
  final UrlLauncher _launch;

  static const Duration _refreshRetryDelay = Duration(seconds: 30);

  AuthStatus _status = AuthStatus.unauthenticated;
  AuthStatus get status => _status;

  /// True when a session exists. The access token may be expired while offline;
  /// the app still shows cached data and reconnects after the next refresh.
  bool get isAuthenticated =>
      _status == AuthStatus.authenticated && _tokens != null;

  TokenData? _tokens;
  TokenData? get tokens => _tokens;

  String? get accessToken => _tokens?.accessToken;

  UserProfile? _userProfile;
  UserProfile? get userProfile => _userProfile;

  String? _lastError;
  String? get lastError => _lastError;

  Timer? _refreshTimer;
  Future<void>? _refreshInFlight;

  PkcePair? _pendingPkce;
  PkcePair? get pendingPkce => _pendingPkce;

  String? _pendingRedirectUri;
  String? get pendingRedirectUri => _pendingRedirectUri;

  final List<Future<void> Function()> _logoutHooks = [];

  AuthController({
    OidcService? oidcService,
    OidcPlatformHelper? platformHelper,
    TokenStorage? tokenStorage,
    UrlLauncher? launcher,
  })  : _oidcService = oidcService ?? OidcService(),
        _platformHelper = platformHelper ?? createPlatformHelper(),
        _tokenStorage = tokenStorage ?? TokenStorage(),
        _launch = launcher ?? _defaultLauncher;

  /// Registers work to run on logout (e.g. clearing the local message cache,
  /// unsubscribing from push).
  void addLogoutHook(Future<void> Function() hook) => _logoutHooks.add(hook);

  /// Restores session state from secure storage. Never waits on the network.
  Future<void> initialize() async {
    try {
      // Web: the app may have been loaded as the redirect target of a login.
      if (_platformHelper.usesFullPageRedirect) {
        final result = await _platformHelper.takeRedirectResult();
        if (result != null) {
          await _completeRedirectLogin(result);
          if (isAuthenticated) return;
        }
      }

      final storedTokens = await _tokenStorage.readTokens();
      if (storedTokens == null || storedTokens.accessToken.isEmpty) {
        return;
      }

      final hasRefreshToken = storedTokens.refreshToken != null &&
          storedTokens.refreshToken!.isNotEmpty;

      if (storedTokens.isExpired && !hasRefreshToken) {
        await _tokenStorage.clearTokens();
        _tokens = null;
        _status = AuthStatus.unauthenticated;
        notifyListeners();
        return;
      }

      // Restore immediately (offline-first); refresh in the background if needed.
      _setSession(storedTokens);
      if (storedTokens.isExpired) {
        unawaited(refreshSession());
      }
    } catch (e) {
      _lastError = 'Session restoration error: $e';
      _tokens = null;
      _status = AuthStatus.unauthenticated;
    }
  }

  /// Checks the current session upon foregrounding.
  Future<void> checkSession() async {
    if (_tokens == null) {
      await initialize();
      return;
    }
    if (_tokens!.isExpired) {
      await refreshSession();
    }
  }

  /// Executes the OAuth 2.0 Authorization Code flow with PKCE.
  ///
  /// Desktop/mobile: opens the system browser and waits for the loopback or deep
  /// link callback. Web: persists PKCE state and navigates this tab to Authelia;
  /// the code is exchanged by [initialize] when the app reloads.
  Future<void> loginWithPurrBrews() async {
    if (_status == AuthStatus.authenticating) return;

    _status = AuthStatus.authenticating;
    _lastError = null;
    notifyListeners();

    var navigatingAway = false;
    try {
      final pkce = PkcePair.generate();
      final redirectUri = AppConfig.authRedirectUri;

      _pendingPkce = pkce;
      _pendingRedirectUri = redirectUri;

      final authUri = await _oidcService.buildAuthorizationUri(
        pkce: pkce,
        redirectUri: redirectUri,
      );

      if (_platformHelper.usesFullPageRedirect) {
        await _platformHelper.savePendingLogin(PendingLogin(
          codeVerifier: pkce.codeVerifier,
          state: pkce.state,
          redirectUri: redirectUri,
        ));
        final launched = await _launch(authUri, sameTab: true);
        if (!launched) {
          throw Exception('Failed to open the sign-in page');
        }
        navigatingAway = true;
        return; // The page unloads; initialize() completes the login.
      }

      // 1. Start listening for incoming redirect callback
      final codeFuture = _platformHelper.listenForAuthCode(
        redirectUri,
        expectedState: pkce.state,
      );

      // 2. Open system browser (Safari / Chrome) in external application mode
      final launched = await _launch(authUri, sameTab: false);
      if (!launched) {
        throw Exception('Failed to open system browser for authentication');
      }

      // 3. Await authorization code
      final code = await codeFuture;
      if (code == null || code.isEmpty) {
        throw Exception('Authentication was cancelled or timed out');
      }

      // 4. Exchange code for access & refresh tokens
      final tokens = await _oidcService.exchangeCodeForToken(
        code: code,
        codeVerifier: pkce.codeVerifier,
        redirectUri: redirectUri,
      );

      // Commit tokens to secure storage before routing to /chat
      await _tokenStorage.saveTokens(tokens);
      _setSession(tokens);
    } catch (e) {
      _lastError = e.toString().replaceAll('Exception: ', '');
      _status = AuthStatus.error;
      notifyListeners();
    } finally {
      if (!navigatingAway) {
        _pendingPkce = null;
        _pendingRedirectUri = null;
        _platformHelper.cancel();
      }
    }
  }

  Future<void> _completeRedirectLogin(RedirectResult result) async {
    if (result.error != null || result.code == null || result.code!.isEmpty) {
      _lastError = 'OIDC Error: ${result.error ?? 'no authorization code'}';
      _status = AuthStatus.error;
      notifyListeners();
      return;
    }
    try {
      final tokens = await _oidcService.exchangeCodeForToken(
        code: result.code!,
        codeVerifier: result.pending.codeVerifier,
        redirectUri: result.pending.redirectUri,
      );
      await _tokenStorage.saveTokens(tokens);
      _setSession(tokens);
    } catch (e) {
      _lastError = e.toString().replaceAll('Exception: ', '');
      _status = AuthStatus.error;
      notifyListeners();
    }
  }

  /// Refreshes the access token using the stored refresh token. Concurrent calls
  /// share one in-flight request.
  Future<void> refreshSession() {
    return _refreshInFlight ??=
        _doRefresh().whenComplete(() => _refreshInFlight = null);
  }

  Future<void> _doRefresh() async {
    final currentRefreshToken = _tokens?.refreshToken;
    if (currentRefreshToken == null || currentRefreshToken.isEmpty) {
      await logout();
      return;
    }

    try {
      final refreshedTokens = await _oidcService.refreshToken(
        refreshToken: currentRefreshToken,
      );
      await _tokenStorage.saveTokens(refreshedTokens);
      _setSession(refreshedTokens);
    } on OidcTokenRejectedException {
      // Refresh token expired or revoked: the session is over.
      await logout();
    } catch (e) {
      // Offline, timeout or server error: keep the session and retry later.
      _lastError = 'Session refresh pending: $e';
      _refreshTimer?.cancel();
      _refreshTimer = Timer(_refreshRetryDelay, () => refreshSession());
    }
  }

  /// Invalidate local tokens and return to unauthenticated state.
  Future<void> logout() async {
    _refreshTimer?.cancel();
    _refreshTimer = null;
    _tokens = null;
    _userProfile = null;
    _pendingPkce = null;
    _pendingRedirectUri = null;
    _status = AuthStatus.unauthenticated;
    _lastError = null;
    try {
      await _tokenStorage.clearTokens();
    } catch (_) {}
    await _runHooks(_logoutHooks);
    notifyListeners();
  }

  Future<void> _runHooks(List<Future<void> Function()> hooks) async {
    for (final hook in List.of(hooks)) {
      try {
        await hook();
      } catch (e) {
        debugPrint('Auth hook failed: $e');
      }
    }
  }

  void _setSession(TokenData tokens) {
    _tokens = tokens;
    _userProfile = UserProfile.fromTokens(
      accessToken: tokens.accessToken,
      idToken: tokens.idToken,
    );
    _status = AuthStatus.authenticated;
    _lastError = null;

    _scheduleRefresh(tokens);
    notifyListeners();
  }

  void _scheduleRefresh(TokenData tokens) {
    _refreshTimer?.cancel();

    // Schedule refresh 60 seconds before expiration
    final refreshDuration =
        tokens.timeUntilExpiry - const Duration(seconds: 60);
    final duration = refreshDuration.isNegative
        ? const Duration(seconds: 5)
        : refreshDuration;

    _refreshTimer = Timer(duration, () {
      refreshSession();
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    _platformHelper.cancel();
    super.dispose();
  }
}
