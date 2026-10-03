/// Platform-agnostic interface for capturing OAuth 2.0 PKCE redirect callbacks.
abstract class OidcPlatformHelper {
  /// Starts listening for an incoming authorization code callback.
  /// On Desktop, this binds an ephemeral loopback HTTP server.
  /// On Web, this inspects browser URL query parameters.
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState});

  /// Releases any active background listeners or loopback servers.
  void cancel();
}

/// Factory constructor implemented by conditional platform exports.
OidcPlatformHelper createPlatformHelper() =>
    throw UnsupportedError('Platform not supported');
