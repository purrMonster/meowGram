import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';

void main() {
  test(
    'reconnect exchanges a fresh ticket and never sends bearer in socket URL',
    () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final sockets = <WebSocket>[];
      final bearerHeaders = <String?>[];
      final tickets = <String>[];
      server.listen((request) async {
        if (request.uri.path == '/ticket') {
          bearerHeaders.add(request.headers.value('authorization'));
          request.response.headers.contentType = ContentType.json;
          request.response.write(
            jsonEncode({'ticket': 'ticket-${bearerHeaders.length}'}),
          );
          await request.response.close();
        } else {
          expect(request.uri.queryParameters.containsKey('token'), isFalse);
          tickets.add(request.uri.queryParameters['ticket']!);
          sockets.add(await WebSocketTransformer.upgrade(request));
        }
      });
      final service = ChatWebSocketService(
        ticketUrl: 'http://127.0.0.1:${server.port}/ticket',
      );
      addTearDown(() async {
        service.dispose();
        for (final socket in sockets) {
          await socket.close();
        }
        await server.close(force: true);
      });
      Future<void> connected() => service.statusStream
          .firstWhere((s) => s == SocketStatus.connected)
          .timeout(const Duration(seconds: 10));
      var ready = connected();
      service.connect(
        customWsUrl: 'ws://127.0.0.1:${server.port}/ws',
        accessToken: 'first',
      );
      await ready;
      ready = connected();
      service.reconnectNow(accessToken: 'second');
      await ready;
      expect(bearerHeaders, ['Bearer first', 'Bearer second']);
      expect(tickets, ['ticket-1', 'ticket-2']);
      expect(service.connectedUrl, 'ws://127.0.0.1:${server.port}/ws');
    },
  );
}
