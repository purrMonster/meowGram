import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/config/app_config.dart';

// Run with --dart-define-from-file=config/production.json (placeholders) or your
// gitignored config/production.local.json. Domains are not asserted literally:
// real values must never be committed (AGENTS.md).
void main() {
  test('AppConfig loads compile-time environment configuration correctly', () {
    if (AppConfig.environment == 'production') {
      expect(AppConfig.appDomain, isNotEmpty);
      expect(AppConfig.port, equals('443'));
      expect(AppConfig.httpPort, equals(443));
      expect(AppConfig.useSecureSchemes, isTrue);
      expect(AppConfig.autheliaClientId, equals('meowgram-client'));
      expect(AppConfig.autheliaIssuerUrl, startsWith('https://'));
      expect(AppConfig.apiBaseUrl, equals('https://${AppConfig.appDomain}'));
      expect(AppConfig.wsBaseUrl, equals('wss://${AppConfig.appDomain}/ws'));
      expect(AppConfig.isDevelopment, isFalse);
    } else {
      expect(AppConfig.environment, equals('development'));
      expect(AppConfig.appDomain, equals('localhost'));
      expect(AppConfig.port, equals('8080'));
      expect(AppConfig.httpPort, equals(8080));
      expect(AppConfig.isDevelopment, isTrue);
    }
  });

  test('no hardcoded Immich host and no client-side Immich key', () {
    expect(AppConfig.immichDomain, isEmpty);
    expect(AppConfig.immichApiUrl, isEmpty);
  });
}
