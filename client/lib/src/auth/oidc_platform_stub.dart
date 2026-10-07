/// State that must survive a full-page redirect to Authelia and back (web).
class PendingLogin {
  final String codeVerifier;
  final String state;
  final String redirectUri;

  const PendingLogin({
    required this.codeVerifier,
    required this.state,
    required this.redirectUri,
  });
}

/// Authorization response picked up after a full-page redirect.
class RedirectResult {
  final String? code;
  final String? error;
  final PendingLogin pending;

  const RedirectResult({this.code, this.error, required this.pending});
}

/// Platform-agnostic interface for capturing OAuth 2.0 PKCE redirect callbacks.
abstract class OidcPlatformHelper {
  /// True when login navigates the current page away (web). The result is then
  /// collected on the next app start via [takeRedirectResult], not by
  /// [listenForAuthCode].
  bool get usesFullPageRedirect;

  /// Starts listening for an incoming authorization code callback.
  /// On Desktop, this binds a loopback HTTP server; on Mobile, it listens for
  /// deep links. Not used on Web.
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState});

  /// Persists PKCE state before a full-page redirect (web only).
  Future<void> savePendingLogin(PendingLogin pending);

  /// Returns the authorization response if the app was loaded as the redirect
  /// target of a login started by this browser tab, and clears it from the URL
  /// and storage. Returns null otherwise (including on a state mismatch).
  Future<RedirectResult?> takeRedirectResult();

  /// Releases any active background listeners or loopback servers.
  void cancel();
}

/// Factory constructor implemented by conditional platform exports.
OidcPlatformHelper createPlatformHelper() =>
    throw UnsupportedError('Platform not supported');
