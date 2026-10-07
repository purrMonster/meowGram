import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:workmanager/workmanager.dart';
import 'package:hive_flutter/hive_flutter.dart';
import 'package:meowgram_client/src/auth/token_storage.dart';
import 'package:meowgram_client/src/config/app_config.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

const String kBackgroundMessageSyncTask = "com.purrbrews.meowgram.syncMessages";

@pragma('vm:entry-point')
void callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    try {
      debugPrint("Background task executing: $task");
      
      // Initialize storage for the isolate
      await Hive.initFlutter();
      
      final tokenStorage = TokenStorage();
      final tokens = await tokenStorage.readTokens();
      if (tokens == null || tokens.accessToken.isEmpty) {
        debugPrint("Background sync aborted: No access token");
        return Future.value(true);
      }
      
      final localRepo = HiveLocalMessageRepository();
      final newest = await localRepo.getNewestMessage();
      if (newest == null) {
        debugPrint("Background sync aborted: No local messages to sync after");
        return Future.value(true);
      }
      
      final afterIso = newest.createdAt.toUtc().toIso8601String();
      final targetSyncUrl = AppConfig.syncUrl();
      final uri = Uri.parse(targetSyncUrl).replace(
        queryParameters: {'after': afterIso},
      );
      
      final response = await http.get(uri, headers: {
        'Accept': 'application/json',
        'Authorization': 'Bearer ${tokens.accessToken}',
      });
      
      if (response.statusCode == 200) {
        final decoded = jsonDecode(response.body);
        final missedMessages = <ChatMessage>[];
        
        if (decoded is List) {
          for (final item in decoded) {
            if (item is Map) {
              missedMessages.add(ChatMessage.fromJson(Map<String, dynamic>.from(item)));
            }
          }
        } else if (decoded is Map && decoded['messages'] is List) {
          for (final item in decoded['messages']) {
            if (item is Map) {
              missedMessages.add(ChatMessage.fromJson(Map<String, dynamic>.from(item)));
            }
          }
        }
        
        if (missedMessages.isNotEmpty) {
          await localRepo.saveMessages(missedMessages);
          debugPrint("Background sync saved ${missedMessages.length} messages");
          // TODO: Fire local notification here if new messages arrived
        }
      } else {
        debugPrint("Background sync failed with status ${response.statusCode}: ${response.body}");
      }
      
      return Future.value(true);
    } catch (e, stack) {
      debugPrint("Background sync error: $e\n$stack");
      return Future.value(false);
    }
  });
}

class BackgroundMessageService {
  static Future<void> initialize() async {
    if (kIsWeb) return; // Workmanager not fully supported on Web
    
    await Workmanager().initialize(
      callbackDispatcher,
      isInDebugMode: kDebugMode,
    );
  }
  
  static Future<void> registerPeriodicSync() async {
    if (kIsWeb) return;
    
    await Workmanager().registerPeriodicTask(
      "meowgram-sync-periodic",
      kBackgroundMessageSyncTask,
      frequency: const Duration(minutes: 15),
      constraints: Constraints(
        networkType: NetworkType.connected,
      ),
    );
  }
}
