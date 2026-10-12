import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/config/server_profile.dart';

/// Which server a published (domain-free) build talks to.
///
/// - [loadCached] is local-only, so it runs before the first frame
///   (offline-first, AGENTS.md).
/// - [refresh] re-reads `/api/client-config` in the background; on failure the
///   cached server stays in use.
/// - [connect] is the first-launch setup on mobile and desktop. The web app
///   never asks: its server is its own origin.
///
/// Builds compiled for one server (`APP_DOMAIN` / `API_BASE_URL` defined) skip
/// all of this.
class ServerConfigController extends ChangeNotifier {
  final ServerProfileStore _store;
  final http.Client? _client;

  bool _loading = false;
  String? _error;

  ServerConfigController({ServerProfileStore? store, http.Client? client})
      : _store = store ?? ServerProfileStore(),
        _client = client;

  bool get isLoading => _loading;
  String? get error => _error;

  /// The app has a server it can sign in against.
  bool get isReady => AppConfig.hasBuiltInServer || AppConfig.server != null;

  bool get needsSetup => AppConfig.needsServerSetup;

  /// Applies the stored server, if any. Never touches the network.
  Future<void> loadCached() async {
    if (AppConfig.hasBuiltInServer) return;
    final cached = await _store.read();
    if (cached != null) AppConfig.useServer(cached);
  }

  /// Re-reads the current server's settings. Returns whether a server is usable.
  Future<bool> refresh() async {
    final base = AppConfig.serverConfigBase;
    if (base == null) return isReady;
    return await _run(() => fetchServerProfile(base, client: _client)) || isReady;
  }

  /// Connects to the address a person typed on the setup screen. Returns
  /// whether that server answered; on failure [error] says why.
  Future<bool> connect(String address) =>
      _run(() => fetchServerProfile(address, client: _client));

  /// Forgets the server so the setup screen asks again.
  Future<void> forget() async {
    AppConfig.useServer(null);
    _error = null;
    await _store.clear();
    notifyListeners();
  }

  Future<bool> _run(Future<ServerProfile> Function() fetch) async {
    _loading = true;
    _error = null;
    notifyListeners();
    try {
      final profile = await fetch();
      AppConfig.useServer(profile);
      await _store.write(profile);
      return true;
    } on ServerConfigException catch (e) {
      _error = e.message;
      return false;
    } catch (e) {
      _error = 'Couldn\'t reach the server.';
      debugPrint('Server configuration error: $e');
      return false;
    } finally {
      _loading = false;
      notifyListeners();
    }
  }
}
