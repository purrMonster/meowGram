import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/oidc_platform.dart';
import 'package:meowgram_client/src/auth/oidc_service.dart';
import 'package:meowgram_client/src/auth/pkce_helper.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:url_launcher/url_launcher.dart';

/// Central authentication state controller managing the OIDC PKCE lifecycle,
/// active token storage, token auto-refresh timers, and user profile state.
class AuthController extends ChangeNotifier {
  final OidcService _oidcService;
  final OidcPlatformHelper _platformHelper;

  AuthStatus _status = AuthStatus.unauthenticated;
  AuthStatus get status => _status;

  bool get isAuthenticated =>
      _status == AuthStatus.authenticated &&
      _tokens != null &&
      !_tokens!.isExpired;

  TokenData? _tokens;
  TokenData? get tokens => _tokens;

  String? get accessToken => _tokens?.accessToken;

  UserProfile? _userProfile;
  UserProfile? get userProfile => _userProfile;

  String? _lastError;
  String? get lastError => _lastError;

  Timer? _refreshTimer;

  PkcePair? _pendingPkce;
  PkcePair? get pendingPkce => _pendingPkce;

  String? _pendingRedirectUri;
  String? get pendingRedirectUri => _pendingRedirectUri;

  AuthController({
    OidcService? oidcService,
    OidcPlatformHelper? platformHelper,
  })  : _oidcService = oidcService ?? OidcService(),
        _platformHelper = platformHelper ?? createPlatformHelper();

  /// Executes the OAuth 2.0 Authorization Code flow with PKCE.
  ///
  /// Flow:
  /// 1. Generates cryptographic PKCE code_verifier, code_challenge (S256), state, and nonce.
  /// 2. Binds platform redirect listener (RFC 8252 loopback server on Desktop, URL inspector on Web, AppLinks on Mobile).
  /// 3. Launches the system browser to the Authelia authorization endpoint using [LaunchMode.externalApplication].
  /// 4. Captures the authorization code callback.
  /// 5. Exchanges the code and code_verifier for an Access Token and Refresh Token.
  /// 6. Decodes user claims and schedules an automated token refresh.
  Future<void> loginWithPurrBrews() async {
    if (_status == AuthStatus.authenticating) return;

    _status = AuthStatus.authenticating;
    _lastError = null;
    notifyListeners();

    try {
      final pkce = PkcePair.generate();
      final redirectUri = AppConfig.authRedirectUri;

      _pendingPkce = pkce;
      _pendingRedirectUri = redirectUri;

      // 1. Start listening for incoming redirect callback
      final codeFuture = _platformHelper.listenForAuthCode(
        redirectUri,
        expectedState: pkce.state,
      );

      // 2. Build authorization URL with PKCE parameters
      final authUri = await _oidcService.buildAuthorizationUri(
        pkce: pkce,
        redirectUri: redirectUri,
      );

      // 3. Open system browser (Safari / Chrome) in external application mode
      final launched = await launchUrl(
        authUri,
        mode: LaunchMode.externalApplication,
      );

      if (!launched) {
        throw Exception('Failed to open system browser for authentication');
      }

      // 4. Await authorization code
      final code = await codeFuture;
      if (code == null || code.isEmpty) {
        throw Exception('Authentication was cancelled or timed out');
      }

      // 5. Exchange code for access & refresh tokens
      final tokens = await _oidcService.exchangeCodeForToken(
        code: code,
        codeVerifier: pkce.codeVerifier,
        redirectUri: redirectUri,
      );

      _setSession(tokens);
    } catch (e) {
      _lastError = e.toString().replaceAll('Exception: ', '');
      _status = AuthStatus.error;
      notifyListeners();
    } finally {
      _pendingPkce = null;
      _pendingRedirectUri = null;
      _platformHelper.cancel();
    }
  }

  /// Automatically refreshes the access token using the stored refresh token.
  Future<void> refreshSession() async {
    final currentRefreshToken = _tokens?.refreshToken;
    if (currentRefreshToken == null || currentRefreshToken.isEmpty) {
      logout();
      return;
    }

    try {
      final refreshedTokens = await _oidcService.refreshToken(
        refreshToken: currentRefreshToken,
      );
      _setSession(refreshedTokens);
    } catch (e) {
      // If refresh fails, invalidate session and force re-login
      logout();
    }
  }

  /// Invalidate local tokens and return to unauthenticated state.
  void logout() {
    _refreshTimer?.cancel();
    _refreshTimer = null;
    _tokens = null;
    _userProfile = null;
    _pendingPkce = null;
    _pendingRedirectUri = null;
    _status = AuthStatus.unauthenticated;
    _lastError = null;
    notifyListeners();
  }

  void _setSession(TokenData tokens) {
    _tokens = tokens;
    _userProfile = UserProfile.fromJwt(tokens.idToken ?? tokens.accessToken);
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
