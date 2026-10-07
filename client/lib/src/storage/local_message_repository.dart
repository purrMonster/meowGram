import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:meowgram_client/src/models/chat_message.dart';

/// Abstract contract for local message persistence.
///
/// Enables seamless offline launch by serving cached messages instantly
/// before the live WebSocket hydration stream completes.
///
/// Only server-confirmed chat messages (a PostgreSQL `id` and type `chat` or
/// `history`) are cached. Join/leave notices, error frames and presence are
/// transient and never stored, so they can't pollute the catch-up sync cursor.
abstract class LocalMessageRepository {
  /// Maximum number of messages kept in the local cache.
  static const int maxCachedMessages = 2000;

  /// Whether a message belongs in the persistent cache.
  static bool isCacheable(ChatMessage m) =>
      m.id != null &&
      m.id!.isNotEmpty &&
      (m.type == 'chat' || m.type == 'history');

  Future<void> init();
  Future<List<ChatMessage>> getCachedMessages();

  /// Newest cached chat message: the catch-up sync cursor.
  Future<ChatMessage?> getNewestMessage();
  Future<void> saveMessage(ChatMessage message);
  Future<void> saveMessages(List<ChatMessage> messages);
  Future<void> clear();
}

/// Hive-backed production implementation of [LocalMessageRepository].
///
/// Storage Mechanics:
/// - Uses a lightweight key-value box (`meowgram_messages_v1`).
/// - Messages are stored keyed by their unique PostgreSQL UUID (`id`),
///   preventing duplication across offline cache and online hydration bursts.
/// - Works cross-platform on Web (IndexedDB), Windows, macOS, Linux, Android, and iOS.
/// - Must only be opened from the UI isolate (Hive is not multi-isolate safe).
class HiveLocalMessageRepository implements LocalMessageRepository {
  static const String boxName = 'meowgram_messages_v1';
  Box<Map<dynamic, dynamic>>? _box;

  @override
  Future<void> init() async {
    if (_box != null && _box!.isOpen) return;
    try {
      _box = await Hive.openBox<Map<dynamic, dynamic>>(boxName);
      await _purgeLegacyEntries();
    } catch (e) {
      // Gracefully catch uninitialized Hive in test environments
      debugPrint('Notice: Hive storage unavailable or uninitialized: $e');
    }
  }

  /// Older builds cached system/error frames under synthetic `local_` keys.
  Future<void> _purgeLegacyEntries() async {
    final legacy =
        _box!.keys.where((k) => k is String && k.startsWith('local_')).toList();
    if (legacy.isNotEmpty) {
      await _box!.deleteAll(legacy);
    }
  }

  List<ChatMessage> _decodeAll() {
    final messages = <ChatMessage>[];
    for (final raw in _box!.values) {
      final msg = ChatMessage.fromJson(Map<String, dynamic>.from(raw));
      if (LocalMessageRepository.isCacheable(msg)) messages.add(msg);
    }
    // Maintain strictly ascending chronological order for timeline rendering
    messages.sort((a, b) => a.createdAt.compareTo(b.createdAt));
    return messages;
  }

  @override
  Future<List<ChatMessage>> getCachedMessages() async {
    try {
      await init();
      if (_box == null || !_box!.isOpen) return [];
      return _decodeAll();
    } catch (e) {
      debugPrint('Warning: Error reading cached messages: $e');
      return [];
    }
  }

  @override
  Future<ChatMessage?> getNewestMessage() async {
    final cached = await getCachedMessages();
    if (cached.isEmpty) return null;
    return cached.last;
  }

  @override
  Future<void> saveMessage(ChatMessage message) => saveMessages([message]);

  @override
  Future<void> saveMessages(List<ChatMessage> messages) async {
    try {
      await init();
      if (_box == null || !_box!.isOpen) return;

      final entries = <String, Map<String, dynamic>>{};
      for (final msg in messages) {
        if (LocalMessageRepository.isCacheable(msg)) {
          entries[msg.id!] = msg.toJson();
        }
      }
      if (entries.isEmpty) return;
      await _box!.putAll(entries);
      await _prune();
    } catch (e) {
      debugPrint('Warning: Failed to cache messages: $e');
    }
  }

  /// Keeps the newest [LocalMessageRepository.maxCachedMessages]. Runs only when
  /// the box overshoots by 10%, so the full decode is rare.
  Future<void> _prune() async {
    const max = LocalMessageRepository.maxCachedMessages;
    if (_box!.length <= max + max ~/ 10) return;
    final all = _decodeAll();
    final excess = all.length - max;
    if (excess > 0) {
      await _box!.deleteAll(all.take(excess).map((m) => m.id));
    }
  }

  @override
  Future<void> clear() async {
    await init();
    if (_box != null && _box!.isOpen) {
      await _box!.clear();
    }
  }
}

/// In-memory implementation of [LocalMessageRepository] for unit/widget tests.
class MemoryLocalMessageRepository implements LocalMessageRepository {
  final Map<String, ChatMessage> _store = {};

  @override
  Future<void> init() async {}

  @override
  Future<List<ChatMessage>> getCachedMessages() async {
    final list = _store.values.toList();
    list.sort((a, b) => a.createdAt.compareTo(b.createdAt));
    return list;
  }

  @override
  Future<ChatMessage?> getNewestMessage() async {
    final list = await getCachedMessages();
    if (list.isEmpty) return null;
    return list.last;
  }

  @override
  Future<void> saveMessage(ChatMessage message) async {
    if (LocalMessageRepository.isCacheable(message)) {
      _store[message.id!] = message;
    }
  }

  @override
  Future<void> saveMessages(List<ChatMessage> messages) async {
    for (final msg in messages) {
      await saveMessage(msg);
    }
  }

  @override
  Future<void> clear() async {
    _store.clear();
  }
}
