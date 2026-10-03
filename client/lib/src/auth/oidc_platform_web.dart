import 'dart:async';

import 'package:meowgram_client/src/auth/oidc_platform_stub.dart';

/// Web implementation of [OidcPlatformHelper].
///
/// Mechanics:
/// On Web, redirect-based OAuth flows return the user to the web application's
/// base URL with query parameters (`?code=...&state=...`). This helper inspects
/// the browser's current [Uri.base] to extract the authorization code.
class OidcPlatformWebHelper implements OidcPlatformHelper {
  @override
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState}) async {
    final currentUri = Uri.base;

    // Check standard query parameters: ?code=xyz&state=abc
    var code = currentUri.queryParameters['code'];
    var state = currentUri.queryParameters['state'];

    // In Flutter Web hash routing (/#/login?code=...), check fragment parameters if query is empty
    if (code == null && currentUri.fragment.contains('?')) {
      final fragmentQueryIndex = currentUri.fragment.indexOf('?');
      final fragmentQuery =
          currentUri.fragment.substring(fragmentQueryIndex + 1);
      final params = Uri.splitQueryString(fragmentQuery);
      code = params['code'];
      state = params['state'];
    }

    if (code != null && code.isNotEmpty) {
      if (state != null && state.isNotEmpty && state != expectedState) {
        return null;
      }
      return code;
    }

    return null;
  }

  @override
  void cancel() {}
}

OidcPlatformHelper createPlatformHelper() => OidcPlatformWebHelper();
