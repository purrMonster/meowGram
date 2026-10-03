import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/main.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/pkce_helper.dart';
import 'package:meowgram_client/src/config/app_config.dart';

void main() {
  group('AppConfig OIDC Contract Tests', () {
    test('Default environment, URLs, and Authelia OIDC contract match defaults',
        () {
      expect(AppConfig.appName, equals('meowGram'));
      expect(AppConfig.environment, equals('development'));
      expect(AppConfig.appDomain, equals('localhost'));
      expect(AppConfig.autheliaClientId, equals('meowgram-client'));
      expect(AppConfig.autheliaIssuerUrl, equals('http://localhost:9091'));
      expect(AppConfig.authRedirectUri, isNotEmpty);
      expect(AppConfig.apiBaseUrl, equals('http://localhost:8080'));
      expect(AppConfig.wsBaseUrl, equals('ws://localhost:8080/ws'));

      final authWs = AppConfig.authenticatedWsUrl('sample_token_xyz');
      expect(authWs, contains('token=sample_token_xyz'));
    });
  });

  group('PKCE Cryptographic Helper Tests', () {
    test('Generates RFC 7636 compliant verifier, challenge, state, and nonce',
        () {
      final pkce = PkcePair.generate();
      expect(pkce.codeVerifier.length, greaterThanOrEqualTo(43));
      expect(pkce.codeChallenge, isNotEmpty);
      expect(pkce.codeChallengeMethod, equals('S256'));
      expect(pkce.state.length, greaterThanOrEqualTo(32));
      expect(pkce.nonce.length, greaterThanOrEqualTo(32));
    });
  });

  group('UserProfile Token Decoding Tests', () {
    test('Parses sub and preferred username from unverified payload preview',
        () {
      // Sample JWT with payload {"sub":"sub-cat-123","preferred_username":"whiskers"}
      const sampleJwt =
          'eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJzdWItY2F0LTEyMyIsInByZWZlcnJlZF91c2VybmFtZSI6IndoaXNrZXJzIn0.dummySignature';
      final profile = UserProfile.fromJwt(sampleJwt);
      expect(profile.sub, equals('sub-cat-123'));
      expect(profile.username, equals('whiskers'));
    });
  });

  group('Widget & Login Screen Tests', () {
    testWidgets('Renders LoginScreen with single "Login with purrBrews" button',
        (
      WidgetTester tester,
    ) async {
      final authController = AuthController();
      await tester.pumpWidget(MeowGramApp(authController: authController));
      await tester.pumpAndSettle();

      // Verify brand title and single login button
      expect(find.text('meowGram'), findsOneWidget);
      expect(find.text('Login with purrBrews'), findsOneWidget);
      expect(find.text('Authelia OIDC & Target Contract'), findsOneWidget);
    });
  });
}
