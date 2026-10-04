import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/token_storage.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/widgets/sidebar.dart';

class FakeSecureStorage extends FlutterSecureStorage {
  final Map<String, String> _store = {};

  @override
  Future<void> write({
    required String key,
    required String? value,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    if (value != null) {
      _store[key] = value;
    } else {
      _store.remove(key);
    }
  }

  @override
  Future<String?> read({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    return _store[key];
  }

  @override
  Future<void> delete({
    required String key,
    AppleOptions? iOptions,
    AndroidOptions? aOptions,
    LinuxOptions? lOptions,
    WebOptions? webOptions,
    AppleOptions? mOptions,
    WindowsOptions? wOptions,
  }) async {
    _store.remove(key);
  }
}

String _createMockJwt(Map<String, dynamic> claims) {
  final header = base64Url.encode(utf8.encode(jsonEncode({'alg': 'RS256', 'typ': 'JWT'}))).replaceAll('=', '');
  final payload = base64Url.encode(utf8.encode(jsonEncode(claims))).replaceAll('=', '');
  const signature = 'mockSignatureBytes';
  return '$header.$payload.$signature';
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('1. Token Persistence & Storage Tests', () {
    test('TokenData serializes and restores exact expiration timestamp', () {
      final now = DateTime.now().toUtc();
      final expires = now.add(const Duration(hours: 2));

      final token = TokenData(
        accessToken: 'access-123',
        idToken: 'id-456',
        refreshToken: 'refresh-789',
        expiresAt: expires,
      );

      final jsonMap = token.toJson();
      final restored = TokenData.fromJson(jsonMap);

      expect(restored.accessToken, 'access-123');
      expect(restored.idToken, 'id-456');
      expect(restored.refreshToken, 'refresh-789');
      expect(restored.isExpired, isFalse);
      expect(restored.expiresAt.difference(expires).inSeconds.abs(), lessThanOrEqualTo(1));
    });

    test('TokenStorage saves, reads, and clears token data securely', () async {
      final fakeStorage = FakeSecureStorage();
      final tokenStorage = TokenStorage(storage: fakeStorage);

      final tokens = TokenData(
        accessToken: 'secret-access-token',
        refreshToken: 'secret-refresh-token',
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      );

      await tokenStorage.saveTokens(tokens);
      final read = await tokenStorage.readTokens();

      expect(read, isNotNull);
      expect(read!.accessToken, 'secret-access-token');
      expect(read.refreshToken, 'secret-refresh-token');

      await tokenStorage.clearTokens();
      final cleared = await tokenStorage.readTokens();
      expect(cleared, isNull);
    });
  });

  group('2. UUID Username & Friendly Name Extraction Tests', () {
    test('Extracts preferred_username from Authelia access token over UUID sub', () {
      final accessToken = _createMockJwt({
        'sub': '141a5dfa-67ef-4e4b-97e3-0599c1581451',
        'preferred_username': 'purrmonster',
        'email': 'purr@whiskertreat.fyi',
      });

      final profile = UserProfile.fromTokens(accessToken: accessToken);
      expect(profile.sub, '141a5dfa-67ef-4e4b-97e3-0599c1581451');
      expect(profile.username, 'purrmonster');
    });

    test('Extracts name when preferred_username is absent', () {
      final accessToken = _createMockJwt({
        'sub': '141a5dfa-uuid',
        'name': 'Lord Mittens',
        'email': 'mittens@whiskertreat.fyi',
      });

      final profile = UserProfile.fromTokens(accessToken: accessToken);
      expect(profile.username, 'Lord Mittens');
    });

    test('Extracts email prefix when name and preferred_username are absent', () {
      final accessToken = _createMockJwt({
        'sub': '141a5dfa-uuid',
        'email': 'whiskers_prime@whiskertreat.fyi',
      });

      final profile = UserProfile.fromTokens(accessToken: accessToken);
      expect(profile.username, 'whiskers_prime');
    });

    test('Falls back to UUID sub only when all display claims are missing', () {
      final accessToken = _createMockJwt({
        'sub': '141a5dfa-raw-uuid',
      });

      final profile = UserProfile.fromTokens(accessToken: accessToken);
      expect(profile.username, '141a5dfa-raw-uuid');
    });
  });

  group('3. Presence Frame Parsing & ChatBloc Integration Tests', () {
    test('ChatMessage parses presence wire format with users array', () {
      final wireJson = {
        'type': 'presence',
        'users': [
          {'username': 'Felix', 'sub': 'sub-felix', 'is_online': true},
          {'username': 'Garfield', 'sub': 'sub-garfield', 'is_online': false},
        ],
        'created_at': DateTime.now().toUtc().toIso8601String(),
      };

      final msg = ChatMessage.fromJson(wireJson);
      expect(msg.isPresence, isTrue);
      expect(msg.users.length, 2);
      expect(msg.users[0].username, 'Felix');
      expect(msg.users[0].isOnline, isTrue);
      expect(msg.users[1].username, 'Garfield');
      expect(msg.users[1].isOnline, isFalse);
    });

    test('ChatBloc updates activeUsers on presence frame and ignores timeline insertion', () async {
      final socketService = ChatWebSocketService();
      final bloc = ChatBloc(socketService: socketService);

      final presenceMsg = ChatMessage(
        type: 'presence',
        textContent: '',
        createdAt: DateTime.now(),
        users: const [
          UserPresence(username: 'Chester', sub: 'sub-chester', isOnline: true),
          UserPresence(username: 'Chloe', sub: 'sub-chloe', isOnline: true),
        ],
      );

      bloc.add(ChatMessageReceived(presenceMsg));

      await expectLater(
        bloc.stream,
        emits(predicate<ChatState>((state) {
          return state.activeUsers.length == 2 &&
              state.activeUsers[0].username == 'Chester' &&
              state.messages.isEmpty &&
              state.isPresenceLoading == false;
        })),
      );

      await bloc.close();
      socketService.dispose();
    });
  });

  group('4. Sidebar Presence UI Tests', () {
    testWidgets('Renders real connected presence members without mock cats', (tester) async {
      tester.view.physicalSize = const Size(1200, 1200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      final authController = AuthController();

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Sidebar(
              authController: authController,
              isPresenceLoading: false,
              activeUsers: const [
                UserPresence(username: 'WhiskersReal', sub: 'sub-1', isOnline: true),
                UserPresence(username: 'ShadowReal', sub: 'sub-2', isOnline: true),
              ],
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      // Ensure mock users are completely absent
      expect(find.text('Mittens'), findsNothing);
      expect(find.text('Felix'), findsNothing);
      expect(find.text('Garfield'), findsNothing);
      expect(find.text('Luna'), findsNothing);

      // Verify real connected users are rendered
      expect(find.text('WhiskersReal'), findsOneWidget);
      expect(find.text('ShadowReal'), findsOneWidget);
      expect(find.text('2 online'), findsOneWidget);
    });

    testWidgets('Renders empty state when no users are online', (tester) async {
      tester.view.physicalSize = const Size(1200, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      final authController = AuthController();

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Sidebar(
              authController: authController,
              isPresenceLoading: false,
              activeUsers: const [],
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('No other cats online yet'), findsOneWidget);
      expect(find.text('0 online'), findsOneWidget);
    });
  });
}
