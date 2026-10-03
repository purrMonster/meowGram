import 'dart:async';
import 'dart:io';

import 'package:meowgram_client/src/auth/oidc_platform_stub.dart';

/// Desktop & IO implementation of [OidcPlatformHelper] implementing RFC 8252 Section 7.3
/// (OAuth 2.0 for Native Apps - Loopback Interface Redirection).
///
/// Mechanics:
/// Native desktop applications on Windows, macOS, and Linux do not share a single browser
/// security context. Per RFC 8252, the application binds an ephemeral or fixed loopback HTTP server
/// on `127.0.0.1`, launches the default system browser to Authelia's authorization endpoint,
/// and awaits the redirect on the loopback port.
class OidcPlatformIoHelper implements OidcPlatformHelper {
  HttpServer? _server;

  @override
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState}) async {
    cancel();

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
            if (!completer.isCompleted) completer.complete(null);
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

      // Automatic timeout after 180 seconds to avoid lingering socket
      Timer(const Duration(seconds: 180), () {
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
  }
}

OidcPlatformHelper createPlatformHelper() => OidcPlatformIoHelper();
