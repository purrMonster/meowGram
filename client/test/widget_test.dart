import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/main.dart';

void main() {
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
  });
}
