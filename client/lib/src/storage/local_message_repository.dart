import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:meowgram_client/src/models/chat_message.dart';

/// Abstract contract for local message persistence.
///
/// Enables seamless offline launch by serving cached messages instantly
/// before the live WebSocket hydration stream completes.
abstract class LocalMessageRepository {
  Future<void> init();
  Future<List<ChatMessage>> getCachedMessages();
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
class HiveLocalMessageRepository implements LocalMessageRepository {
  static const String boxName = 'meowgram_messages_v1';
  Box<Map>? _box;

  @override
  Future<void> init() async {
    if (_box != null && _box!.isOpen) return;
    try {
      _box = await Hive.openBox<Map>(boxName);
    } catch (e) {
      // Gracefully catch uninitialized Hive in test environments
      debugPrint('Notice: Hive storage unavailable or uninitialized: $e');
    }
  }

  @override
  Future<List<ChatMessage>> getCachedMessages() async {
    try {
      await init();
      if (_box == null || !_box!.isOpen) return [];

      final messages = <ChatMessage>[];
      for (final raw in _box!.values) {
        if (raw is Map) {
          final msg = ChatMessage.fromJson(Map<String, dynamic>.from(raw));
          messages.add(msg);
        }
      }

      // Maintain strictly ascending chronological order for timeline rendering
      messages.sort((a, b) => a.createdAt.compareTo(b.createdAt));
      return messages;
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
  Future<void> saveMessage(ChatMessage message) async {
    try {
      await init();
      if (_box == null || !_box!.isOpen) return;

      // Use unique PostgreSQL UUID if available, fallback to millisecond timestamp
      final key = message.id ??
          'local_${message.createdAt.millisecondsSinceEpoch}_${message.hashCode}';
      await _box!.put(key, message.toJson());
    } catch (e) {
      debugPrint('Warning: Failed to cache message locally: $e');
    }
  }

  @override
  Future<void> saveMessages(List<ChatMessage> messages) async {
    try {
      await init();
      if (_box == null || !_box!.isOpen || messages.isEmpty) return;

      final entries = <String, Map<String, dynamic>>{};
      for (final msg in messages) {
        final key = msg.id ??
            'local_${msg.createdAt.millisecondsSinceEpoch}_${msg.hashCode}';
        entries[key] = msg.toJson();
      }
      await _box!.putAll(entries);
    } catch (e) {
      debugPrint('Warning: Failed to batch cache messages: $e');
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
    final key = message.id ??
        'mem_${message.createdAt.millisecondsSinceEpoch}_${message.hashCode}';
    _store[key] = message;
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
