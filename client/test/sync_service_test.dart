import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/services/sync_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('SyncService Tests (UST-1.4.3)', () {
    late MemoryLocalMessageRepository memoryRepo;
    late ChatWebSocketService socketService;

    setUp(() {
      memoryRepo = MemoryLocalMessageRepository();
      socketService = ChatWebSocketService();
    });

    tearDown(() {
      socketService.dispose();
    });

    test('Queries newest local cached message and builds ISO 8601 UTC request',
        () async {
      // Seed repository with older messages
      final msg1 = ChatMessage(
        id: 'msg-1',
        senderId: 'sub-alice',
        username: 'alice',
        textContent: 'First message',
        createdAt: DateTime.utc(2026, 10, 3, 10, 0, 0),
      );
      final msg2 = ChatMessage(
        id: 'msg-2',
        senderId: 'sub-bob',
        username: 'bob',
        textContent: 'Second message',
        createdAt: DateTime.utc(2026, 10, 3, 10, 15, 0),
      );
      await memoryRepo.saveMessages([msg1, msg2]);

      String? capturedUrl;
      Map<String, String>? capturedHeaders;

      final mockClient = MockClient((request) async {
        capturedUrl = request.url.toString();
        capturedHeaders = request.headers;

        final responseJson = jsonEncode([
          {
            'id': 'msg-3',
            'sender_id': 'sub-charlie',
            'username': 'charlie',
            'text_content': 'Missed offline message',
            'created_at': '2026-10-03T10:30:00.000Z',
          }
        ]);
        return http.Response(responseJson, 200, headers: {
          'content-type': 'application/json; charset=utf-8',
        });
      });

      final syncService = SyncService(
        socketService: socketService,
        localRepo: memoryRepo,
        httpClient: mockClient,
        baseUrlOverride: 'http://test.meowgram.local:8080',
        accessToken: 'test-jwt-token-xyz',
      );

      final result = await syncService.sync();

      expect(result.length, 1);
      expect(result.first.id, 'msg-3');
      expect(result.first.textContent, 'Missed offline message');

      // Verify URL contains ISO 8601 UTC timestamp of msg2 (the newest cached message)
      expect(capturedUrl, contains('/api/messages/sync'));
      expect(capturedUrl, contains('after=2026-10-03T10%3A15%3A00.000Z'));

      // Verify Authorization Bearer header
      expect(capturedHeaders?['authorization'], 'Bearer test-jwt-token-xyz');

      syncService.dispose();
    });

    test('Skips REST sync if local cache holds no messages (empty state)',
        () async {
      var requestMade = false;
      final mockClient = MockClient((request) async {
        requestMade = true;
        return http.Response('[]', 200);
      });

      final syncService = SyncService(
        socketService: socketService,
        localRepo: memoryRepo, // empty
        httpClient: mockClient,
      );

      final result = await syncService.sync();

      expect(result, isEmpty);
      expect(requestMade, isFalse);

      syncService.dispose();
    });

    test('Emits synchronized messages via onSyncCompleted broadcast stream',
        () async {
      await memoryRepo.saveMessage(ChatMessage(
        id: 'msg-local',
        senderId: 'sub-me',
        textContent: 'Hello',
        createdAt: DateTime.utc(2026, 10, 3, 12, 0, 0),
      ));

      final mockClient = MockClient((request) async {
        return http.Response(
          jsonEncode([
            {
              'id': 'msg-catchup',
              'sender_id': 'sub-peer',
              'text_content': 'Caught up',
              'created_at': '2026-10-03T12:05:00.000Z',
            }
          ]),
          200,
        );
      });

      final syncService = SyncService(
        socketService: socketService,
        localRepo: memoryRepo,
        httpClient: mockClient,
      );

      expectLater(
        syncService.onSyncCompleted,
        emits(predicate<List<ChatMessage>>((list) {
          return list.length == 1 && list.first.id == 'msg-catchup';
        })),
      );

      await syncService.sync();
      syncService.dispose();
    });
  });

  group('ChatBloc Sync Integration Tests (UST-1.4.3)', () {
    late MemoryLocalMessageRepository memoryRepo;
    late ChatWebSocketService socketService;

    setUp(() {
      memoryRepo = MemoryLocalMessageRepository();
      socketService = ChatWebSocketService();
    });

    tearDown(() {
      socketService.dispose();
    });

    test(
        'Merges catch-up messages into active state and local cache with UUID deduplication',
        () async {
      // 1. Setup initial state with cached message
      final existingMsg = ChatMessage(
        id: 'uuid-1',
        senderId: 'sub-alice',
        username: 'alice',
        textContent: 'Existing message',
        createdAt: DateTime.utc(2026, 10, 3, 10, 0, 0),
      );
      await memoryRepo.saveMessage(existingMsg);

      final bloc = ChatBloc(
        socketService: socketService,
        localRepo: memoryRepo,
      );

      // Load cache into bloc state
      bloc.add(const ChatInitializeRequested());
      await Future<void>.delayed(const Duration(milliseconds: 50));

      expect(bloc.state.messages.length, 1);
      expect(bloc.state.messages.first.id, 'uuid-1');

      // 2. Dispatch SyncCompleted with 1 duplicate and 2 new missed messages
      final duplicateMsg = ChatMessage(
        id: 'uuid-1', // duplicate
        senderId: 'sub-alice',
        username: 'alice',
        textContent: 'Existing message',
        createdAt: DateTime.utc(2026, 10, 3, 10, 0, 0),
      );
      final missedMsg1 = ChatMessage(
        id: 'uuid-2',
        senderId: 'sub-bob',
        username: 'bob',
        textContent: 'Missed while offline 1',
        createdAt: DateTime.utc(2026, 10, 3, 10, 05, 0),
      );
      final missedMsg2 = ChatMessage(
        id: 'uuid-3',
        senderId: 'sub-charlie',
        username: 'charlie',
        textContent: 'Missed while offline 2',
        createdAt: DateTime.utc(2026, 10, 3, 10, 10, 0),
      );

      bloc.add(SyncCompleted([duplicateMsg, missedMsg2, missedMsg1]));
      await Future<void>.delayed(const Duration(milliseconds: 50));

      // 3. Verify deduplication and strict chronological ordering
      expect(bloc.state.messages.length, 3);
      expect(bloc.state.messages[0].id, 'uuid-1');
      expect(bloc.state.messages[1].id, 'uuid-2');
      expect(bloc.state.messages[2].id, 'uuid-3');

      // 4. Verify new messages were persisted to local repository
      final cachedAfterSync = await memoryRepo.getCachedMessages();
      expect(cachedAfterSync.length, 3);
      expect(cachedAfterSync.map((m) => m.id).toList(), [
        'uuid-1',
        'uuid-2',
        'uuid-3',
      ]);

      await bloc.close();
    });

    test(
        'Handles overlapping hydration burst and catch-up sync without duplication',
        () async {
      final bloc = ChatBloc(
        socketService: socketService,
        localRepo: memoryRepo,
      );

      // WebSocket live hydration pushes msg-A
      final msgA = ChatMessage(
        id: 'uuid-A',
        senderId: 'sub-1',
        textContent: 'Live Burst Message',
        createdAt: DateTime.utc(2026, 10, 3, 11, 0, 0),
      );
      bloc.add(ChatMessageReceived(msgA));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      // Catch-up sync also returned msg-A (overlap) plus msg-B (gap fill)
      final msgB = ChatMessage(
        id: 'uuid-B',
        senderId: 'sub-2',
        textContent: 'Older Gap Message',
        createdAt: DateTime.utc(2026, 10, 3, 10, 50, 0), // chronologically before A
      );
      bloc.add(SyncCompleted([msgB, msgA]));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      expect(bloc.state.messages.length, 2);
      // Verify chronological order: msg-B (10:50) before msg-A (11:00)
      expect(bloc.state.messages[0].id, 'uuid-B');
      expect(bloc.state.messages[1].id, 'uuid-A');

      await bloc.close();
    });
  });
}
