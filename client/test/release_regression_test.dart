// Regression tests for the 1.0.1 stability release (see runbook §12).
import 'dart:async';
import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';
import 'package:meowgram_client/src/auth/oidc_platform.dart';
import 'package:meowgram_client/src/auth/oidc_service.dart';
import 'package:meowgram_client/src/auth/token_storage.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/services/background_message_service.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/services/sync_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

class MemSecureStorage extends FlutterSecureStorage {
  final Map<String, String> store = {};

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
    if (value == null) {
      store.remove(key);
    } else {
      store[key] = value;
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
  }) async =>
      store[key];

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
    store.remove(key);
  }
}

class FakeRedirectHelper implements OidcPlatformHelper {
  FakeRedirectHelper({this.fullPage = false, this.redirect});
  final bool fullPage;
  RedirectResult? redirect;
  PendingLogin? saved;

  @override
  bool get usesFullPageRedirect => fullPage;

  @override
  Future<String?> listenForAuthCode(String redirectUri,
          {required String expectedState}) async =>
      null;

  @override
  Future<void> savePendingLogin(PendingLogin pending) async => saved = pending;

  @override
  Future<RedirectResult?> takeRedirectResult() async {
    final r = redirect;
    redirect = null;
    return r;
  }

  @override
  void cancel() {}
}

String jwt(Map<String, dynamic> claims) {
  String seg(Map<String, dynamic> m) =>
      base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
  return '${seg({'alg': 'RS256'})}.${seg(claims)}.sig';
}

TokenData expiredTokens() => TokenData(
      accessToken: jwt({'sub': 'me', 'preferred_username': 'whiskers'}),
      refreshToken: 'refresh-1',
      expiresAt: DateTime.now().subtract(const Duration(hours: 1)),
    );

String tokenResponse({String access = 'new-access', String? refresh}) =>
    jsonEncode({
      'access_token': access,
      'expires_in': 3600,
      if (refresh != null) 'refresh_token': refresh,
    });

/// MockClient that 404s discovery (falls back to configured endpoints) and
/// delegates token requests to [onToken].
MockClient oidcClient(Future<http.Response> Function(http.Request) onToken) =>
    MockClient((req) async {
      if (req.method == 'GET') return http.Response('not found', 404);
      return onToken(req);
    });

ChatMessage chat(String id, DateTime at,
        {String type = 'chat', String sender = 'other'}) =>
    ChatMessage(
      id: id,
      senderId: sender,
      username: sender,
      textContent: 'msg $id',
      createdAt: at,
      type: type,
    );

class FakeSocket extends ChatWebSocketService {
  final StreamController<ChatMessage> msgs =
      StreamController<ChatMessage>.broadcast();
  final StreamController<SocketStatus> statuses =
      StreamController<SocketStatus>.broadcast();
  SocketStatus _s = SocketStatus.disconnected;

  @override
  SocketStatus get status => _s;
  @override
  Stream<ChatMessage> get messageStream => msgs.stream;
  @override
  Stream<SocketStatus> get statusStream => statuses.stream;
  @override
  void connect({String? customWsUrl, String? accessToken}) {}

  void emitStatus(SocketStatus s) {
    _s = s;
    statuses.add(s);
  }
}

void main() {
  group('AuthController offline-first session handling', () {
    test('restores an expired session immediately and keeps it when offline',
        () async {
      final storage = TokenStorage(storage: MemSecureStorage());
      await storage.saveTokens(expiredTokens());
      final auth = AuthController(
        oidcService: OidcService(
          httpClient: oidcClient((_) async => throw const SocketTimeoutLike()),
        ),
        platformHelper: FakeRedirectHelper(),
        tokenStorage: storage,
      );

      await auth.initialize();
      expect(auth.isAuthenticated, isTrue,
          reason: 'cached session must open without network');

      await auth.refreshSession();
      expect(auth.isAuthenticated, isTrue,
          reason: 'a network failure must not log the user out');
      expect(await storage.readTokens(), isNotNull);
      auth.dispose();
    });

    test('logs out only when Authelia rejects the refresh token', () async {
      final storage = TokenStorage(storage: MemSecureStorage());
      await storage.saveTokens(expiredTokens());
      var hookRan = false;
      final auth = AuthController(
        oidcService: OidcService(
          httpClient: oidcClient(
              (_) async => http.Response('{"error":"invalid_grant"}', 400)),
        ),
        platformHelper: FakeRedirectHelper(),
        tokenStorage: storage,
      )..addLogoutHook(() async => hookRan = true);

      await auth.initialize();
      await auth.refreshSession();

      expect(auth.isAuthenticated, isFalse);
      expect(await storage.readTokens(), isNull);
      expect(hookRan, isTrue, reason: 'logout hooks clear cache and push');
      auth.dispose();
    });

    test('concurrent refreshes share a single token request', () async {
      final storage = TokenStorage(storage: MemSecureStorage());
      await storage.saveTokens(expiredTokens());
      var tokenCalls = 0;
      final gate = Completer<void>();
      final auth = AuthController(
        oidcService: OidcService(
          httpClient: oidcClient((_) async {
            tokenCalls++;
            await gate.future;
            return http.Response(tokenResponse(refresh: 'refresh-2'), 200);
          }),
        ),
        platformHelper: FakeRedirectHelper(),
        tokenStorage: storage,
      );

      await auth.initialize(); // kicks off one background refresh
      final a = auth.refreshSession();
      final b = auth.refreshSession();
      gate.complete();
      await Future.wait([a, b]);

      expect(tokenCalls, 1);
      expect(auth.accessToken, 'new-access');
      expect((await storage.readTokens())!.refreshToken, 'refresh-2');
      auth.dispose();
    });

    test('web: completes login from the redirect on startup', () async {
      String? sentVerifier;
      final auth = AuthController(
        oidcService: OidcService(
          httpClient: oidcClient((req) async {
            sentVerifier = req.bodyFields['code_verifier'];
            return http.Response(tokenResponse(refresh: 'r'), 200);
          }),
        ),
        platformHelper: FakeRedirectHelper(
          fullPage: true,
          redirect: const RedirectResult(
            code: 'the-code',
            pending: PendingLogin(
              codeVerifier: 'stored-verifier',
              state: 's',
              redirectUri: 'https://meow.example.home.arpa',
            ),
          ),
        ),
        tokenStorage: TokenStorage(storage: MemSecureStorage()),
      );

      await auth.initialize();
      expect(auth.isAuthenticated, isTrue);
      expect(sentVerifier, 'stored-verifier');
      auth.dispose();
    });

    test('web: login persists PKCE state and navigates the same tab', () async {
      final helper = FakeRedirectHelper(fullPage: true);
      bool? usedSameTab;
      final auth = AuthController(
        oidcService: OidcService(httpClient: oidcClient((_) async {
          fail('no token request expected before redirect');
        })),
        platformHelper: helper,
        tokenStorage: TokenStorage(storage: MemSecureStorage()),
        launcher: (uri, {required bool sameTab}) async {
          usedSameTab = sameTab;
          return true;
        },
      );

      await auth.loginWithPurrBrews();
      expect(usedSameTab, isTrue);
      expect(helper.saved, isNotNull);
      expect(helper.saved!.codeVerifier.length, greaterThanOrEqualTo(43));
      auth.dispose();
    });
  });

  group('Token never shown in UI', () {
    test('redactToken strips credential query parameters', () {
      expect(
        ChatWebSocketService.redactToken(
            'wss://meow.example.home.arpa/ws?token=eyJ.secret.jwt'),
        'wss://meow.example.home.arpa/ws',
      );
      expect(
        ChatWebSocketService.redactToken('ws://h/ws?x=1&token=abc'),
        'ws://h/ws?x=1',
      );
      expect(
        ChatWebSocketService.redactToken('wss://h/ws?ticket=short-lived'),
        'wss://h/ws',
      );
    });
  });

  group('Local cache hygiene', () {
    test('system, error and id-less frames are never cached', () async {
      final repo = MemoryLocalMessageRepository();
      final t = DateTime.utc(2026, 10, 7);
      await repo.saveMessages([
        chat('a', t),
        chat('h', t.add(const Duration(seconds: 1)), type: 'history'),
        ChatMessage(textContent: 'x pounced', type: 'system', createdAt: t),
        ChatMessage(textContent: 'oops', type: 'error', createdAt: t),
      ]);
      final cached = await repo.getCachedMessages();
      expect(cached.map((m) => m.id), ['a', 'h']);
    });
  });

  group('Catch-up sync', () {
    test('pages through gaps larger than one server page', () async {
      final socket = FakeSocket();
      final repo = MemoryLocalMessageRepository();
      final start = DateTime.utc(2026, 10, 7, 8);
      await repo.saveMessage(chat('seed', start));

      final afters = <String>[];
      final client = MockClient((req) async {
        afters.add(req.url.queryParameters['after']!);
        final after = DateTime.parse(req.url.queryParameters['after']!);
        final remaining = 1200 - afters.length * 0; // total messages available
        final offset = after.difference(start).inSeconds;
        final count =
            (remaining - offset).clamp(0, SyncService.pageSize).toInt();
        final page = [
          for (var i = 1; i <= count; i++)
            chat('m${offset + i}', start.add(Duration(seconds: offset + i)))
                .toJson()
        ];
        return http.Response(jsonEncode(page), 200);
      });

      final sync = SyncService(
          socketService: socket, localRepo: repo, httpClient: client);
      final got = await sync.sync();

      expect(got.length, 1200);
      expect(afters.length, 3, reason: '500 + 500 + 200');
      sync.dispose();
    });

    test('cursor is captured before the history burst refills the cache',
        () async {
      final socket = FakeSocket();
      final repo = MemoryLocalMessageRepository();
      final old = DateTime.utc(2026, 10, 7, 8);
      await repo.saveMessage(chat('old', old));

      String? requestedAfter;
      final client = MockClient((req) async {
        requestedAfter = req.url.queryParameters['after'];
        return http.Response('[]', 200);
      });
      final sync = SyncService(
          socketService: socket, localRepo: repo, httpClient: client);

      await sync.captureCursor(); // before connecting
      // History burst lands in the cache before the sync request goes out.
      await repo.saveMessage(chat('fresh', DateTime.utc(2026, 10, 7, 12)));
      socket.emitStatus(SocketStatus.connected);
      await Future<void>.delayed(const Duration(milliseconds: 50));

      expect(requestedAfter, old.toIso8601String());
      sync.dispose();
    });
  });

  group('ChatBloc', () {
    test('notifies only for live chat from other people', () async {
      final socket = FakeSocket();
      final notified = <String>[];
      final bloc = ChatBloc(
        socketService: socket,
        localRepo: MemoryLocalMessageRepository(),
        currentSubProvider: () => 'me',
        notifier: (m) async => notified.add(m.id!),
      );
      bloc.add(const ChatInitializeRequested(accessToken: 't'));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      final t = DateTime.utc(2026, 10, 7);
      socket.msgs
        ..add(chat('h1', t, type: 'history'))
        ..add(ChatMessage(textContent: 'x joined', type: 'system', createdAt: t))
        ..add(chat('mine', t, sender: 'me'))
        ..add(chat('theirs', t, sender: 'other'));
      await Future<void>.delayed(const Duration(milliseconds: 50));

      expect(notified, ['theirs']);
      await bloc.close();
    });

    test('initialization is idempotent (layout + screen both request it)',
        () async {
      final socket = FakeSocket();
      final bloc = ChatBloc(
        socketService: socket,
        localRepo: MemoryLocalMessageRepository(),
        notifier: (_) async {},
      );
      bloc
        ..add(const ChatInitializeRequested(accessToken: 't'))
        ..add(const ChatInitializeRequested(accessToken: 't'));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      socket.msgs.add(chat('one', DateTime.utc(2026, 10, 7)));
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(bloc.state.messages.length, 1,
          reason: 'a duplicate subscription would deliver it twice');
      await bloc.close();
    });

    test('same-length roster changes are emitted', () async {
      final socket = FakeSocket();
      final bloc = ChatBloc(
        socketService: socket,
        localRepo: MemoryLocalMessageRepository(),
        notifier: (_) async {},
      );
      bloc.add(const ChatInitializeRequested(accessToken: 't'));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      ChatMessage roster(String name) => ChatMessage(
            textContent: '',
            type: 'presence',
            createdAt: DateTime.now(),
            users: [UserPresence(username: name, sub: name)],
          );
      socket.msgs.add(roster('mittens'));
      await Future<void>.delayed(const Duration(milliseconds: 20));
      socket.msgs.add(roster('felix'));
      await Future<void>.delayed(const Duration(milliseconds: 20));

      expect(bloc.state.activeUsers.single.username, 'felix');
      await bloc.close();
    });
  });

  group('Background check (opt-in)', () {
    test('first run only records a cursor; later runs notify for others',
        () async {
      final tokens = TokenStorage(storage: MemSecureStorage());
      await tokens.saveTokens(TokenData(
        accessToken: jwt({'sub': 'me'}),
        expiresAt: DateTime.now().add(const Duration(hours: 1)),
      ));
      final cursorStore = MemSecureStorage();
      var requests = 0;
      final client = MockClient((req) async {
        requests++;
        return http.Response(
          jsonEncode([
            chat('1', DateTime.utc(2026, 10, 7, 9), sender: 'me').toJson(),
            chat('2', DateTime.utc(2026, 10, 7, 10), sender: 'felix').toJson(),
          ]),
          200,
        );
      });
      int? notifiedCount;

      Future<void> run() => runBackgroundCheck(
            tokenStorage: tokens,
            cursorStorage: cursorStore,
            httpClient: client,
            notify: (count, latest) async => notifiedCount = count,
          );

      await run();
      expect(requests, 0);
      expect(cursorStore.store, isNotEmpty);

      await run();
      expect(requests, 1);
      expect(notifiedCount, 1, reason: 'own message excluded');
      expect(cursorStore.store.values.single, '2026-10-07T10:00:00.000Z');
    });
  });
}

/// A stand-in for a network failure (not an OidcTokenRejectedException).
class SocketTimeoutLike implements Exception {
  const SocketTimeoutLike();
}
