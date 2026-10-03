import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/main.dart';

void main() {
  group('AppConfig Tests', () {
    test('Default environment and URLs match contract', () {
      expect(AppConfig.appName, equals('meowGram'));
      expect(AppConfig.environment, equals('development'));
      expect(AppConfig.appDomain, equals('localhost'));
      expect(AppConfig.apiBaseUrl, equals('http://localhost:8080'));
      expect(AppConfig.wsBaseUrl, equals('ws://localhost:8080/ws'));
      expect(AppConfig.healthCheckUrl, equals('http://localhost:8080/healthz'));
    });
  });

  group('Widget & Navigation Tests', () {
    testWidgets('Renders meowGram login screen with brand and enter button', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(const MeowGramApp());
      await tester.pumpAndSettle();

      // Verify brand title and login elements
      expect(find.text('meowGram'), findsOneWidget);
      expect(find.text('Enter Chat Lounge'), findsOneWidget);
      expect(find.byType(TextField), findsOneWidget);
      expect(find.text('Environment Target Contract'), findsOneWidget);

      // Tap Enter Chat Lounge button to verify navigation to /chat
      await tester.tap(find.text('Enter Chat Lounge'));
      await tester.pumpAndSettle();

      // Verify chat lounge rendered
      expect(find.text('meowGram Lounge'), findsOneWidget);
      expect(find.textContaining('Connected to Go Echo Backend'), findsOneWidget);
    });
  });
}
