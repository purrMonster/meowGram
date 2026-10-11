import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/services/sync_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

void main() {
  test(
    '429 retries the same UUID page and preserves the completed page',
    () async {
      final socket = ChatWebSocketService();
      final repo = MemoryLocalMessageRepository();
      final at = DateTime.utc(2026, 1, 1);
      await repo.saveMessage(
        ChatMessage(
          id: '00000000-0000-0000-0000-000000000000',
          createdAt: at,
          textContent: 'seed',
        ),
      );
      final cursors = <String?>[];
      final sync = SyncService(
        socketService: socket,
        localRepo: repo,
        httpClient: MockClient((request) async {
          cursors.add(request.url.queryParameters['after_id']);
          if (cursors.length == 1) {
            return http.Response(
              jsonEncode(
                List.generate(
                  500,
                  (i) => {
                    'id':
                        '00000000-0000-0000-0000-${(i + 1).toString().padLeft(12, '0')}',
                    'created_at': at.toIso8601String(),
                    'text_content': 'scratch',
                  },
                ),
              ),
              200,
            );
          }
          if (cursors.length == 2) {
            return http.Response('', 429, headers: {'retry-after': '1'});
          }
          return http.Response('[]', 200);
        }),
      );
      addTearDown(() {
        sync.dispose();
        socket.dispose();
      });
      final messages = await sync.sync();
      expect(messages.length, 500);
      expect(cursors.length, 3);
      expect(cursors[1], '00000000-0000-0000-0000-000000000500');
      expect(cursors[2], cursors[1]);
    },
  );
}
