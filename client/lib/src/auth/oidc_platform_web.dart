import 'dart:async';

import 'package:meowgram_client/src/auth/oidc_platform_stub.dart';
import 'package:web/web.dart' as web;

/// Web implementation of [OidcPlatformHelper].
///
/// Mechanics:
/// The login navigates this tab to Authelia (same tab), so all in-memory state is
/// lost. The PKCE verifier, state and redirect URI are kept in `sessionStorage`
/// (per-tab, cleared when the tab closes). When Authelia redirects back with
/// `?code=…&state=…`, [takeRedirectResult] matches the state, returns the code,
/// removes the stored values and strips the query from the address bar.
class OidcPlatformWebHelper implements OidcPlatformHelper {
  static const _kVerifier = 'meowgram.oidc.verifier';
  static const _kState = 'meowgram.oidc.state';
  static const _kRedirect = 'meowgram.oidc.redirect';

  @override
  bool get usesFullPageRedirect => true;

  @override
  Future<String?> listenForAuthCode(String redirectUri,
      {required String expectedState}) async {
    // Not used on web: see takeRedirectResult.
    return null;
  }

  @override
  Future<void> savePendingLogin(PendingLogin pending) async {
    final storage = web.window.sessionStorage;
    storage.setItem(_kVerifier, pending.codeVerifier);
    storage.setItem(_kState, pending.state);
    storage.setItem(_kRedirect, pending.redirectUri);
  }

  @override
  Future<RedirectResult?> takeRedirectResult() async {
    final storage = web.window.sessionStorage;
    final verifier = storage.getItem(_kVerifier);
    final expectedState = storage.getItem(_kState);
    final redirectUri = storage.getItem(_kRedirect);

    final params = _callbackParams(Uri.base);
    final hasCallback = params.containsKey('code') || params.containsKey('error');
    if (!hasCallback) return null;

    // A callback is present: always clean up so a reload can't replay it.
    storage.removeItem(_kVerifier);
    storage.removeItem(_kState);
    storage.removeItem(_kRedirect);
    _stripCallbackFromAddressBar();

    if (verifier == null || expectedState == null || redirectUri == null) {
      return null; // Not started from this tab.
    }
    if (params['state'] != expectedState) {
      return null; // Missing or mismatched state: possible CSRF, ignore.
    }

    return RedirectResult(
      code: params['code'],
      error: params['error'],
      pending: PendingLogin(
        codeVerifier: verifier,
        state: expectedState,
        redirectUri: redirectUri,
      ),
    );
  }

  Map<String, String> _callbackParams(Uri uri) {
    if (uri.queryParameters.isNotEmpty) return uri.queryParameters;
    // Hash routing (/#/login?code=...)
    final fragment = uri.fragment;
    final i = fragment.indexOf('?');
    if (i >= 0) return Uri.splitQueryString(fragment.substring(i + 1));
    return const {};
  }

  void _stripCallbackFromAddressBar() {
    final uri = Uri.base;
    final fragment = uri.fragment;
    final i = fragment.indexOf('?');
    final cleanFragment = i >= 0 ? fragment.substring(0, i) : fragment;
    final clean = '${uri.scheme}://${uri.authority}${uri.path}'
        '${cleanFragment.isNotEmpty ? '#$cleanFragment' : ''}';
    web.window.history.replaceState(null, '', clean);
  }

  @override
  void cancel() {}
}

OidcPlatformHelper createPlatformHelper() => OidcPlatformWebHelper();
