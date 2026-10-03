import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/config/app_config.dart';

void main() {
  test('AppConfig loads compile-time environment configuration correctly', () {
    if (AppConfig.environment == 'production') {
      expect(AppConfig.appDomain, equals('meowgram.purrbrews.cc'));
      expect(AppConfig.port, equals('443'));
      expect(AppConfig.httpPort, equals(443));
      expect(AppConfig.autheliaClientId, equals('meowgram-client'));
      expect(AppConfig.autheliaIssuerUrl, equals('https://auth.purrbrews.cc'));
      expect(AppConfig.apiBaseUrl, equals('https://meowgram.purrbrews.cc'));
      expect(AppConfig.wsBaseUrl, equals('wss://meowgram.purrbrews.cc/ws'));
      expect(AppConfig.isDevelopment, isFalse);
    } else {
      expect(AppConfig.environment, equals('development'));
      expect(AppConfig.appDomain, equals('localhost'));
      expect(AppConfig.port, equals('8080'));
      expect(AppConfig.httpPort, equals(8080));
      expect(AppConfig.isDevelopment, isTrue);
    }
  });
}
