import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/screens/chat_screen.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';
import 'package:meowgram_client/src/widgets/chat_input_bar.dart';
import 'package:meowgram_client/src/widgets/message_bubble.dart';

/// Fake WebSocket service for deterministic unit and widget testing.
class FakeWebSocketService extends ChatWebSocketService {
  final StreamController<ChatMessage> _fakeMsgController =
      StreamController<ChatMessage>.broadcast();
  final StreamController<SocketStatus> _fakeStatusController =
      StreamController<SocketStatus>.broadcast();
  final List<String> sentPayloads = [];

  SocketStatus _fakeStatus = SocketStatus.connected;

  @override
  SocketStatus get status => _fakeStatus;

  @override
  Stream<ChatMessage> get messageStream => _fakeMsgController.stream;

  @override
  Stream<SocketStatus> get statusStream => _fakeStatusController.stream;

  @override
  void connect({String? customWsUrl, String? accessToken}) {
    _fakeStatus = SocketStatus.connected;
    _fakeStatusController.add(_fakeStatus);
  }

  @override
  void sendMessage(String text) {
    sentPayloads.add(text);
  }

  void emitMessage(ChatMessage message) {
    _fakeMsgController.add(message);
  }

  void emitStatus(SocketStatus newStatus) {
    _fakeStatus = newStatus;
    _fakeStatusController.add(newStatus);
  }

  @override
  void dispose() {
    _fakeMsgController.close();
    _fakeStatusController.close();
    super.dispose();
  }
}

void main() {
  group('ChatMessage Model Tests', () {
    test('Correctly parses JSON wire format from WebSocket', () {
      final json = {
        'id': 'msg-uuid-123',
        'sender_id': 'sub-whiskers-99',
        'username': 'whiskers',
        'text_content': 'Meow world!',
        'type': 'chat',
        'created_at': '2026-10-03T12:00:00Z',
      };

      final msg = ChatMessage.fromJson(json);
      expect(msg.id, equals('msg-uuid-123'));
      expect(msg.senderId, equals('sub-whiskers-99'));
      expect(msg.username, equals('whiskers'));
      expect(msg.textContent, equals('Meow world!'));
      expect(msg.type, equals('chat'));
      expect(msg.isFromSelf('sub-whiskers-99'), isTrue);
      expect(msg.isFromSelf('sub-other-user'), isFalse);
      expect(msg.isSystem, isFalse);
      expect(msg.isError, isFalse);
      expect(msg.formattedTime, isNotEmpty);
    });

    test('Identifies system and error message types', () {
      final sysMsg = ChatMessage(
        textContent: 'Mittens joined the lounge! 🐾',
        type: 'system',
        createdAt: DateTime.now(),
      );
      expect(sysMsg.isSystem, isTrue);

      final errorMsg = ChatMessage(
        textContent: 'Database persistence failed',
        type: 'error',
        createdAt: DateTime.now(),
      );
      expect(errorMsg.isError, isTrue);
    });
  });

  group('ChatBloc Logic & Cache-to-Live Handoff Tests', () {
    late FakeWebSocketService fakeService;
    late MemoryLocalMessageRepository fakeLocalRepo;
    late ChatBloc chatBloc;

    setUp(() {
      fakeService = FakeWebSocketService();
      fakeLocalRepo = MemoryLocalMessageRepository();
      chatBloc = ChatBloc(
        socketService: fakeService,
        localRepo: fakeLocalRepo,
      );
    });

    tearDown(() {
      chatBloc.close();
      fakeService.dispose();
    });

    test('Initial connection request hooks into stream and sets connected status',
        () async {
      chatBloc.add(const ChatConnectRequested(accessToken: 'dummy_token'));
      // Allow microtask loop to process the event
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(chatBloc.state.status, equals(SocketStatus.connected));
    });

    test('Immediately renders cached messages before live connection completes',
        () async {
      // 1. Pre-populate local cache
      final cachedMsg = ChatMessage(
        id: 'cached-id-1',
        senderId: 'user-whiskers',
        textContent: 'Offline cached message',
        createdAt: DateTime.parse('2026-10-01T10:00:00Z'),
      );
      await fakeLocalRepo.saveMessage(cachedMsg);

      // 2. Initialize Bloc
      chatBloc.add(const ChatInitializeRequested(accessToken: 'dummy_token'));
      await Future<void>.delayed(const Duration(milliseconds: 10));

      // UI state should immediately reflect cached message
      expect(chatBloc.state.messages.length, equals(1));
      expect(chatBloc.state.messages.first.id, equals('cached-id-1'));
      expect(chatBloc.state.isLoadedFromCache, isTrue);

      // 3. Server emits fresh live message
      final liveMsg = ChatMessage(
        id: 'live-id-2',
        senderId: 'user-felix',
        textContent: 'Fresh live incoming message',
        createdAt: DateTime.parse('2026-10-01T10:05:00Z'),
      );
      fakeService.emitMessage(liveMsg);
      await Future<void>.delayed(const Duration(milliseconds: 10));

      // State merges both cached and live messages
      expect(chatBloc.state.messages.length, equals(2));
      expect(chatBloc.state.messages.last.id, equals('live-id-2'));

      // Background cache also recorded the live message
      final cachedList = await fakeLocalRepo.getCachedMessages();
      expect(cachedList.length, equals(2));
    });

    test('Appends and chronologically sorts messages while deduplicating by ID',
        () async {
      chatBloc.add(const ChatConnectRequested());
      await Future<void>.delayed(const Duration(milliseconds: 10));

      final baseTime = DateTime.parse('2026-10-03T12:00:00Z');

      // 1. Send first message
      final msg1 = ChatMessage(
        id: 'id-1',
        senderId: 'user-a',
        textContent: 'First message',
        createdAt: baseTime,
      );
      fakeService.emitMessage(msg1);
      await Future<void>.delayed(const Duration(milliseconds: 10));

      // 2. Send duplicate of first message (should be ignored)
      fakeService.emitMessage(msg1);
      await Future<void>.delayed(const Duration(milliseconds: 10));

      // 3. Send second newer message
      final msg2 = ChatMessage(
        id: 'id-2',
        senderId: 'user-b',
        textContent: 'Second message',
        createdAt: baseTime.add(const Duration(minutes: 1)),
      );
      fakeService.emitMessage(msg2);
      await Future<void>.delayed(const Duration(milliseconds: 10));

      expect(chatBloc.state.messages.length, equals(2));
      expect(chatBloc.state.messages.first.id, equals('id-1'));
      expect(chatBloc.state.messages.last.id, equals('id-2'));
    });

    test('Dispatches message sending to underlying socket service', () async {
      chatBloc.add(const ChatSendMessage('Test meow'));
      await Future<void>.delayed(const Duration(milliseconds: 10));
      expect(fakeService.sentPayloads, contains('Test meow'));
    });
  });

  group('MessageBubble Widget Tests', () {
    testWidgets('Renders own message right-aligned with primary styling',
        (WidgetTester tester) async {
      final msg = ChatMessage(
        id: 'msg-1',
        senderId: 'my-sub-123',
        username: 'whiskers',
        textContent: 'My own message',
        createdAt: DateTime.now(),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: MessageBubble(
              message: msg,
              currentSub: 'my-sub-123',
            ),
          ),
        ),
      );

      expect(find.text('My own message'), findsOneWidget);
      // Sender label should be omitted for self to reduce noise
      expect(find.text('@whiskers'), findsNothing);
      expect(find.byIcon(Icons.done_all_rounded), findsOneWidget);

      final align = tester.widget<Align>(find.byType(Align));
      expect(align.alignment, equals(Alignment.centerRight));
    });

    testWidgets('Renders peer message left-aligned with sender username label',
        (WidgetTester tester) async {
      final msg = ChatMessage(
        id: 'msg-2',
        senderId: 'peer-sub-456',
        username: 'felix',
        textContent: 'Hello from peer!',
        createdAt: DateTime.now(),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: MessageBubble(
              message: msg,
              currentSub: 'my-sub-123',
            ),
          ),
        ),
      );

      expect(find.text('Hello from peer!'), findsOneWidget);
      expect(find.text('@felix'), findsOneWidget);

      final align = tester.widget<Align>(find.byType(Align));
      expect(align.alignment, equals(Alignment.centerLeft));
    });

    testWidgets('Renders system notice as centered pill',
        (WidgetTester tester) async {
      final msg = ChatMessage(
        textContent: 'Tom entered the room 🐾',
        type: 'system',
        createdAt: DateTime.now(),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: MessageBubble(
              message: msg,
              currentSub: 'my-sub-123',
            ),
          ),
        ),
      );

      expect(find.text('Tom entered the room 🐾'), findsOneWidget);
      expect(find.byType(Center), findsWidgets);
    });
  });

  group('ChatInputBar Widget Tests', () {
    testWidgets('Handles input, typing state, and submit triggers',
        (WidgetTester tester) async {
      String? sentMessage;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ChatInputBar(
              isConnected: true,
              onSendMessage: (val) => sentMessage = val,
            ),
          ),
        ),
      );

      // Initially empty text field
      final textField = find.byType(TextField);
      expect(textField, findsOneWidget);

      // Typing in text field
      await tester.enterText(textField, 'Meow from testing suite!');
      await tester.pump();

      // Find and press send button
      final sendButton = find.byIcon(Icons.send_rounded);
      expect(sendButton, findsOneWidget);
      await tester.tap(sendButton);
      await tester.pump();

      expect(sentMessage, equals('Meow from testing suite!'));
    });
  });

  group('ChatScreen Integration Tests', () {
    testWidgets('Renders lounge room header, messages list, and quick action chips',
        (WidgetTester tester) async {
      final authController = AuthController();
      final fakeService = FakeWebSocketService();
      final fakeLocalRepo = MemoryLocalMessageRepository();
      final chatBloc = ChatBloc(
        socketService: fakeService,
        localRepo: fakeLocalRepo,
      );

      await tester.pumpWidget(
        MaterialApp(
          home: ChatScreen(
            authController: authController,
            socketService: fakeService,
            chatBloc: chatBloc,
            activeRoom: 'general-lounge',
          ),
        ),
      );

      await tester.pump();

      expect(find.text('# general-lounge'), findsOneWidget);
      expect(find.byType(ChatInputBar), findsOneWidget);
      expect(find.text('Purr... 😺'), findsWidgets);

      // Emit a message via socket service
      fakeService.emitMessage(
        ChatMessage(
          id: 'test-burst-1',
          senderId: 'lounge-cat',
          username: 'lounge_cat',
          textContent: 'Welcome to the lounge!',
          createdAt: DateTime.now(),
        ),
      );

      await tester.pumpAndSettle();
      expect(find.text('Welcome to the lounge!'), findsOneWidget);

      chatBloc.close();
      fakeService.dispose();
    });
  });
}
