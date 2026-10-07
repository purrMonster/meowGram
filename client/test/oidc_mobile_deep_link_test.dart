import 'dart:async';
import 'package:app_links/app_links.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/auth/oidc_platform_io.dart';
import 'package:meowgram_client/src/config/app_config.dart';

class FakeAppLinks implements AppLinks {
  final StreamController<Uri> _uriStreamController =
      StreamController<Uri>.broadcast();
  Uri? initialLink;
  Uri? latestLink;

  void emitUri(Uri uri) {
    _uriStreamController.add(uri);
  }

  @override
  Stream<Uri> get uriLinkStream => _uriStreamController.stream;

  @override
  Stream<String> get stringLinkStream =>
      _uriStreamController.stream.map((u) => u.toString());

  @override
  Future<Uri?> getInitialLink() async => initialLink;

  @override
  Future<String?> getInitialLinkString() async => initialLink?.toString();

  @override
  Future<Uri?> getLatestLink() async => latestLink;

  @override
  Future<String?> getLatestLinkString() async => latestLink?.toString();

  void dispose() {
    _uriStreamController.close();
  }
}

void main() {
  group('Mobile Deep Link Interception Tests (OidcPlatformIoHelper)', () {
    late FakeAppLinks fakeAppLinks;
    late OidcPlatformIoHelper helper;

    setUp(() {
      fakeAppLinks = FakeAppLinks();
      helper = OidcPlatformIoHelper(
        appLinks: fakeAppLinks,
        isMobileOverride: true,
      );
    });

    tearDown(() {
      helper.cancel();
      fakeAppLinks.dispose();
    });

    test('extracts authorization code when deep link matches expected state',
        () async {
      const redirectUri = 'meowgram://callback';
      const expectedState = 'test_crypto_state_xyz_123';

      final codeFuture = helper.listenForAuthCode(
        redirectUri,
        expectedState: expectedState,
      );

      // Simulate Authelia redirecting back via meowgram:// custom URL scheme
      fakeAppLinks.emitUri(Uri.parse(
          'meowgram://callback?code=mock_oauth_code_456&state=$expectedState'));

      final code = await codeFuture;
      expect(code, equals('mock_oauth_code_456'));
    });

    test('ignores incoming URIs with mismatched state parameters (CSRF protection)',
        () async {
      const redirectUri = 'meowgram://callback';
      const expectedState = 'authentic_state_123';

      final codeFuture = helper.listenForAuthCode(
        redirectUri,
        expectedState: expectedState,
      );

      // 1. Emit an unrelated or stale deep link from previous session
      fakeAppLinks.emitUri(Uri.parse(
          'meowgram://callback?code=stale_code&state=stale_state_old'));

      // 2. Then emit the legitimate callback
      fakeAppLinks.emitUri(Uri.parse(
          'meowgram://callback?code=valid_code&state=$expectedState'));

      final code = await codeFuture;
      expect(code, equals('valid_code'));
    });

    test('surfaces the Authelia error to the caller (shown in the UI)', () async {
      const redirectUri = 'meowgram://callback';
      const expectedState = 'test_state_err';

      final codeFuture = helper.listenForAuthCode(
        redirectUri,
        expectedState: expectedState,
      );

      fakeAppLinks.emitUri(Uri.parse(
          'meowgram://callback?error=access_denied&error_description=User+cancelled&state=$expectedState'));

      await expectLater(
        codeFuture,
        throwsA(predicate((e) => e.toString().contains('access_denied'))),
      );
    });

    test('extracts authorization code from latest/initial link if available',
        () async {
      const redirectUri = 'meowgram://callback';
      const expectedState = 'cold_launch_state_789';

      fakeAppLinks.latestLink = Uri.parse(
          'meowgram://callback?code=initial_launch_code&state=$expectedState');

      final code = await helper.listenForAuthCode(
        redirectUri,
        expectedState: expectedState,
      );

      expect(code, equals('initial_launch_code'));
    });
  });

  group('AppConfig Mobile Redirect URI Resolution', () {
    test('AppConfig resolves meowgram://callback on mobile platforms', () {
      debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
      expect(AppConfig.authRedirectUri, equals('meowgram://callback'));

      debugDefaultTargetPlatformOverride = TargetPlatform.android;
      expect(AppConfig.authRedirectUri, equals('meowgram://callback'));

      debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
      expect(AppConfig.authRedirectUri, equals('http://127.0.0.1:8088/callback'));

      debugDefaultTargetPlatformOverride = TargetPlatform.windows;
      expect(AppConfig.authRedirectUri, equals('http://127.0.0.1:8088/callback'));

      debugDefaultTargetPlatformOverride = null;
    });
  });
}
