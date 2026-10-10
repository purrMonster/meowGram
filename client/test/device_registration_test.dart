import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:meowgram_client/src/push/device_registration_client.dart';

void main() {
  test(
    'register and revoke use authenticated JSON without URL credentials',
    () async {
      final calls = <String>[];
      final registry = DeviceRegistrationClient(
        endpoint: 'http://localhost/devices',
        client: MockClient((request) async {
          calls.add(request.method);
          expect(request.headers['authorization'], 'Bearer scratch-bearer');
          expect(request.url.query, isEmpty);
          expect(jsonDecode(request.body), {'token': 'scratch-device'});
          return http.Response('', 204);
        }),
      );
      await registry.register('scratch-device', 'scratch-bearer');
      await registry.unregister('scratch-device', 'scratch-bearer');
      expect(calls, ['PUT', 'DELETE']);
    },
  );
  test(
    'refuses empty authentication and reports provider failures without body',
    () async {
      final registry = DeviceRegistrationClient(
        client: MockClient((_) async => http.Response('sensitive-body', 401)),
      );
      await expectLater(registry.register('device', ''), throwsStateError);
      await expectLater(
        registry.register('device', 'bearer'),
        throwsA(
          predicate(
            (e) => e is StateError && !e.toString().contains('sensitive-body'),
          ),
        ),
      );
    },
  );
}
