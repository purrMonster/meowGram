import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/widgets/message_bubble.dart';

void main() {
  Map<String, dynamic> payload() => {
    'id': 'message-uuid',
    'sender_id': 'same-id',
    'username': 'Backup',
    'text_content': 'Finished',
    'created_at': '2026-10-17T00:00:00Z',
  };

  test('sender kind survives cache and separates identical human/bot IDs', () {
    final human = ChatMessage.fromJson(payload());
    final bot = ChatMessage.fromJson({...payload(), 'sender_kind': 'bot'});
    expect(human.isFromSelf('same-id'), isTrue);
    expect(bot.isFromSelf('same-id'), isFalse);
    expect(bot == human, isFalse);
    final restored = ChatMessage.fromJson(bot.toJson());
    expect(restored, bot);
    expect(restored.isFromSelf('same-id'), isFalse);
    expect(
      ChatMessage.fromJson({
        ...payload(),
        'sender_kind': 'unknown',
      }).isFromSelf('same-id'),
      isFalse,
    );
  });

  testWidgets('bot with colliding subject is labeled as a peer', (
    tester,
  ) async {
    final bot = ChatMessage.fromJson({...payload(), 'sender_kind': 'bot'});
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MessageBubble(message: bot, currentSub: 'same-id'),
        ),
      ),
    );
    expect(find.text('@Backup · Bot'), findsOneWidget);
    expect(find.text('You'), findsNothing);
    final align = tester.widget<Align>(
      find
          .ancestor(of: find.text('Finished'), matching: find.byType(Align))
          .first,
    );
    expect(align.alignment, Alignment.centerLeft);
  });
}
