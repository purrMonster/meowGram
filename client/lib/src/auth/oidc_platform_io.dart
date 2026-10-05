import 'dart:async';
import 'dart:io';

import 'package:app_links/app_links.dart';
import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/auth/oidc_platform_stub.dart';

/// Desktop & Mobile IO implementation of [OidcPlatformHelper].
///
/// Mechanics:
/// - Desktop (macOS, Windows, Linux): Implements RFC 8252 Section 7.3 loopback interface
///   redirection by binding an ephemeral or fixed loopback HTTP server on `127.0.0.1:8088`.
/// - Mobile (iOS, Android): Implements native deep link interception using [AppLinks]
///   to capture `meowgram://` custom URL scheme callbacks from the system browser (Safari/Chrome).
class OidcPlatformIoHelper implements OidcPlatformHelper {
  HttpServer? _server;
  StreamSubscription<Uri>? _linkSubscription;
  final AppLinks _appLinks;
  final bool? _isMobileOverride;

  OidcPlatformIoHelper({
    AppLinks? appLinks,
    bool? isMobileOverride,
  })  : _appLinks = appLinks ?? AppLinks(),
        _isMobileOverride = isMobileOverride;

  bool get _isMobile =>
      _isMobileOverride ?? (Platform.isIOS || Platform.isAndroid);

  @override
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState}) async {
    cancel();

    if (_isMobile) {
      return _listenForMobileAuthCode(
        redirectUri,
        expectedState: expectedState,
      );
    }

    return _listenForDesktopAuthCode(
      redirectUri,
      expectedState: expectedState,
    );
  }

  /// Mobile deep link listener using [AppLinks].
  ///
  /// Intercepts `meowgram://callback?code=...&state=...` returned by Authelia via the system browser.
  Future<String?> _listenForMobileAuthCode(
    String redirectUri, {
    required String expectedState,
  }) async {
    final completer = Completer<String?>();
    final targetUri = Uri.parse(redirectUri);

    void handleIncomingUri(Uri uri) {
      final isMeowgramScheme = uri.scheme.toLowerCase() == 'meowgram';
      final matchesTargetScheme =
          uri.scheme.toLowerCase() == targetUri.scheme.toLowerCase();

      // Only inspect URIs matching either the app scheme or explicit redirectUri scheme
      if (!isMeowgramScheme && !matchesTargetScheme) return;

      final query = uri.queryParameters.isNotEmpty
          ? uri.queryParameters
          : (uri.hasFragment
              ? Uri.splitQueryString(uri.fragment)
              : const <String, String>{});

      final state = query['state'];

      // Crucial: Only process callbacks matching this specific PKCE session's state.
      // Ignore stale links from previous sessions or unrelated intents.
      if (state != expectedState) {
        return;
      }

      final error = query['error'];
      if (error != null) {
        debugPrint('Mobile OIDC auth error returned: $error');
        if (!completer.isCompleted) completer.completeError(Exception('OIDC Error: $error'));
        cancel();
        return;
      }

      final code = query['code'];
      if (code != null && code.isNotEmpty) {
        if (!completer.isCompleted) completer.complete(code);
        cancel();
        return;
      }

      if (!completer.isCompleted) completer.complete(null);
      cancel();
    }

    // 1. Subscribe to real-time incoming deep links
    _linkSubscription = _appLinks.uriLinkStream.listen(
      (Uri uri) {
        handleIncomingUri(uri);
      },
      onError: (Object err) {
        debugPrint('AppLinks stream error: $err');
      },
    );

    // 2. Also check if the app was launched directly with the redirect URI
    try {
      final latestLink = await _appLinks.getLatestLink();
      if (latestLink != null) {
        handleIncomingUri(latestLink);
      } else {
        final initialLink = await _appLinks.getInitialLink();
        if (initialLink != null) {
          handleIncomingUri(initialLink);
        }
      }
    } catch (e) {
      debugPrint('AppLinks initial link inspection notice: $e');
    }

    // Timeout after 600 seconds (10 minutes) to prevent lingering resources
    Timer(const Duration(seconds: 600), () {
      if (!completer.isCompleted) {
        completer.complete(null);
        cancel();
      }
    });

    return await completer.future;
  }

  /// Desktop RFC 8252 loopback HTTP listener.
  Future<String?> _listenForDesktopAuthCode(
    String redirectUri, {
    required String expectedState,
  }) async {
    final uri = Uri.parse(redirectUri);
    final port = uri.hasPort ? uri.port : 8088;
    final path = uri.path.isEmpty ? '/callback' : uri.path;

    final completer = Completer<String?>();

    try {
      // Bind to IPv4 loopback (127.0.0.1) per RFC 8252
      _server = await HttpServer.bind(InternetAddress.loopbackIPv4, port);

      final subscription = _server!.listen((HttpRequest request) async {
        if (request.uri.path == path) {
          final query = request.uri.queryParameters;
          final code = query['code'];
          final state = query['state'];
          final error = query['error'];

          if (error != null) {
            _respondWithError(request, error);
            if (!completer.isCompleted) completer.completeError(Exception('OIDC Error: $error'));
            cancel();
            return;
          }

          if (state != expectedState) {
            _respondWithError(
                request, 'Invalid state parameter (potential CSRF attempt)');
            if (!completer.isCompleted) completer.complete(null);
            cancel();
            return;
          }

          if (code != null && code.isNotEmpty) {
            _respondWithSuccess(request);
            if (!completer.isCompleted) completer.complete(code);
            cancel();
            return;
          }

          _respondWithError(request, 'Missing authorization code');
          if (!completer.isCompleted) completer.complete(null);
          cancel();
        } else {
          request.response.statusCode = HttpStatus.notFound;
          await request.response.close();
        }
      });

      // Automatic timeout after 600 seconds (10 minutes) to avoid lingering socket
      Timer(const Duration(seconds: 600), () {
        if (!completer.isCompleted) {
          subscription.cancel();
          completer.complete(null);
          cancel();
        }
      });

      return await completer.future;
    } catch (e) {
      cancel();
      return null;
    }
  }

  void _respondWithSuccess(HttpRequest request) {
    request.response
      ..statusCode = HttpStatus.ok
      ..headers.contentType = ContentType.html
      ..write('''
<!DOCTYPE html>
<html>
  <head>
    <meta charset="utf-8">
    <title>meowGram Authentication</title>
    <style>
      body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        background: #13101C;
        color: #E2DDF5;
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100vh;
        margin: 0;
      }
      .card {
        background: #201A30;
        border: 1px solid #362E4F;
        border-radius: 16px;
        padding: 40px;
        text-align: center;
        max-width: 420px;
        box-shadow: 0 10px 30px rgba(0,0,0,0.5);
      }
      h1 { font-size: 24px; color: #D0BCFF; margin-bottom: 8px; }
      p { font-size: 14px; color: #CAC4D0; line-height: 1.5; }
      .badge { display: inline-block; padding: 6px 14px; background: #381E72; color: #EADDFF; border-radius: 20px; font-size: 12px; margin-top: 16px; }
    </style>
  </head>
  <body>
    <div class="card">
      <div style="font-size: 48px; margin-bottom: 12px;">🐾</div>
      <h1>Authentication Successful!</h1>
      <p>Your session with <strong>purrBrews Authelia</strong> is verified. You can now close this tab and return to meowGram.</p>
      <div class="badge">Session Authorized</div>
    </div>
  </body>
</html>
''');
    request.response.close();
  }

  void _respondWithError(HttpRequest request, String errorMsg) {
    request.response
      ..statusCode = HttpStatus.badRequest
      ..headers.contentType = ContentType.html
      ..write('''
<!DOCTYPE html>
<html>
  <head><title>Authentication Failed</title></head>
  <body style="font-family:sans-serif; text-align:center; padding: 40px; background:#1e1a2e; color:#ffb4ab;">
    <h2>Authentication Error</h2>
    <p>$errorMsg</p>
  </body>
</html>
''');
    request.response.close();
  }

  @override
  void cancel() {
    _server?.close(force: true);
    _server = null;
    _linkSubscription?.cancel();
    _linkSubscription = null;
  }
}

OidcPlatformHelper createPlatformHelper() => OidcPlatformIoHelper();
