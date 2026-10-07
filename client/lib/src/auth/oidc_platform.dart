export 'oidc_platform_stub.dart'
    show OidcPlatformHelper, PendingLogin, RedirectResult;
export 'oidc_platform_stub.dart'
    if (dart.library.io) 'oidc_platform_io.dart'
    if (dart.library.js_interop) 'oidc_platform_web.dart';
