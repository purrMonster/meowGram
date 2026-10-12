import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:meowgram_client/main.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/config/server_config_controller.dart';
import 'package:meowgram_client/src/config/server_profile.dart';

/// In-memory stand-in for the platform keychain.
class _MemStorage extends FlutterSecureStorage {
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
  }) async =>
      store.remove(key);
}

const _config = {
  'issuer': 'https://authelia.example.home.arpa/',
  'client_id': 'meowgram',
  'ws_path': '/ws',
  'ws_ticket_path': '/api/ws-ticket',
  'sync_path': '/api/messages/sync',
  'push_devices_path': '/api/push/devices',
  'health_path': '/healthz',
};

MockClient _server({Map<String, Object> body = _config, int status = 200}) =>
    MockClient((req) async {
      if (req.url.path != kClientConfigPath) return http.Response('', 404);
      return http.Response(jsonEncode(body), status);
    });

void main() {
  tearDown(() => AppConfig.useServer(null));

  group('Server address', () {
    test('defaults to https and keeps only the origin', () {
      expect(ServerProfile.normalizeAddress(' meow.example.home.arpa '),
          'https://meow.example.home.arpa');
      expect(
          ServerProfile.normalizeAddress(
              'https://meow.example.home.arpa:8443/chat?x=1'),
          'https://meow.example.home.arpa:8443');
      expect(ServerProfile.normalizeAddress('https://meow.example.home.arpa:443'),
          'https://meow.example.home.arpa');
    });

    test('allows plain http only for this machine', () {
      expect(ServerProfile.normalizeAddress('localhost:8080'),
          'http://localhost:8080');
      expect(() => ServerProfile.normalizeAddress('http://meow.example.home.arpa'),
          throwsA(isA<ServerConfigException>()));
      expect(() => ServerProfile.normalizeAddress(''),
          throwsA(isA<ServerConfigException>()));
    });
  });

  group('Client config', () {
    test('rejects a server without usable sign-in settings', () {
      expect(
          () => ServerProfile.fromClientConfig(
              'https://meow.example.home.arpa', {'issuer': '', 'client_id': 'x'}),
          throwsA(isA<ServerConfigException>()));
      expect(
          () => ServerProfile.fromClientConfig('https://meow.example.home.arpa',
              {'issuer': 'http://authelia.example.home.arpa', 'client_id': 'x'}),
          throwsA(isA<ServerConfigException>()),
          reason: 'a remote issuer must use https');
    });

    test('fetches the profile from the server', () async {
      final profile = await fetchServerProfile('meow.example.home.arpa',
          client: _server());
      expect(profile.baseUrl, 'https://meow.example.home.arpa');
      expect(profile.issuer, 'https://authelia.example.home.arpa');
      expect(profile.wsUrl, 'wss://meow.example.home.arpa/ws');
    });

    test('explains a non-meowGram answer', () async {
      expect(
        () => fetchServerProfile('meow.example.home.arpa',
            client: _server(status: 404)),
        throwsA(isA<ServerConfigException>().having(
            (e) => e.message, 'message', contains('HTTP 404'))),
      );
    });
  });

  group('ServerConfigController', () {
    test('connect applies and stores the server; AppConfig follows it',
        () async {
      final storage = _MemStorage();
      final controller = ServerConfigController(
          store: ServerProfileStore(storage: storage), client: _server());
      expect(controller.needsSetup, isTrue);

      expect(await controller.connect('meow.example.home.arpa'), isTrue);
      expect(controller.needsSetup, isFalse);
      expect(storage.store, isNotEmpty);
      expect(AppConfig.appDomain, 'meow.example.home.arpa');
      expect(AppConfig.apiBaseUrl, 'https://meow.example.home.arpa');
      expect(AppConfig.wsBaseUrl, 'wss://meow.example.home.arpa/ws');
      expect(AppConfig.autheliaIssuerUrl, 'https://authelia.example.home.arpa');
      expect(AppConfig.autheliaClientId, 'meowgram');
      expect(AppConfig.autheliaDiscoveryUrl,
          'https://authelia.example.home.arpa/.well-known/openid-configuration');
      expect(AppConfig.wsTicketUrl,
          'https://meow.example.home.arpa/api/ws-ticket');
      expect(AppConfig.syncUrl(),
          'https://meow.example.home.arpa/api/messages/sync');
      expect(AppConfig.pushDeviceUrl,
          'https://meow.example.home.arpa/api/push/devices');

      // A fresh start (or the background isolate) finds it again.
      AppConfig.useServer(null);
      await ServerConfigController(store: ServerProfileStore(storage: storage))
          .loadCached();
      expect(AppConfig.server?.baseUrl, 'https://meow.example.home.arpa');
    });

    test('a failed connect leaves setup open and says why', () async {
      final controller = ServerConfigController(
          store: ServerProfileStore(storage: _MemStorage()),
          client: _server(status: 404));
      expect(await controller.connect('meow.example.home.arpa'), isFalse);
      expect(controller.needsSetup, isTrue);
      expect(controller.error, contains('HTTP 404'));
    });

    test('forget clears the stored server', () async {
      final storage = _MemStorage();
      final controller = ServerConfigController(
          store: ServerProfileStore(storage: storage), client: _server());
      await controller.connect('meow.example.home.arpa');
      await controller.forget();
      expect(controller.needsSetup, isTrue);
      expect(storage.store, isEmpty);
    });
  });

  group('First launch', () {
    testWidgets('asks for the server, then shows sign-in', (tester) async {
      final controller = ServerConfigController(
          store: ServerProfileStore(storage: _MemStorage()), client: _server());
      await tester.pumpWidget(MeowGramApp(
        authController: AuthController(),
        serverConfig: controller,
      ));
      await tester.pumpAndSettle();

      expect(find.text('Connect to your lounge'), findsOneWidget);
      await tester.enterText(
          find.byKey(const Key('server-address')), 'meow.example.home.arpa');
      await tester.tap(find.text('Continue'));
      await tester.pumpAndSettle();

      expect(find.text('Login with purrBrews'), findsOneWidget);
      expect(find.text('Change server'), findsOneWidget);
      expect(find.text('https://authelia.example.home.arpa'), findsOneWidget);
    });
  });
}
