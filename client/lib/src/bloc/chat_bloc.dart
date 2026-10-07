import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/services/sync_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';
import 'package:meowgram_client/src/services/notification_service.dart';

// =============================================================================
// Chat Events
// =============================================================================

abstract class ChatEvent {
  const ChatEvent();
}

/// Request to initialize the chat timeline with local cache before connecting live.
class ChatInitializeRequested extends ChatEvent {
  final String? accessToken;
  final String? customWsUrl;

  const ChatInitializeRequested({this.accessToken, this.customWsUrl});
}

/// Request to connect to the backend WebSocket endpoint.
class ChatConnectRequested extends ChatEvent {
  final String? accessToken;
  final String? customWsUrl;

  const ChatConnectRequested({this.accessToken, this.customWsUrl});
}

/// Dispatched whenever a new [ChatMessage] is emitted by the WebSocket service.
class ChatMessageReceived extends ChatEvent {
  final ChatMessage message;

  const ChatMessageReceived(this.message);
}

/// Dispatched when catch-up synchronization delivers missed messages (UST-1.4.3).
class SyncCompleted extends ChatEvent {
  final List<ChatMessage> messages;

  const SyncCompleted(this.messages);
}

/// Request to send a message to the lounge.
class ChatSendMessage extends ChatEvent {
  final String text;

  const ChatSendMessage(this.text);
}

/// Dispatched when the underlying WebSocket status changes.
class ChatStatusChanged extends ChatEvent {
  final SocketStatus status;
  final String? error;

  const ChatStatusChanged(this.status, [this.error]);
}

/// Request to disconnect from the WebSocket service.
class ChatDisconnectRequested extends ChatEvent {
  const ChatDisconnectRequested();
}

// =============================================================================
// Chat State
// =============================================================================

class ChatState {
  final SocketStatus status;
  final List<ChatMessage> messages;
  final List<UserPresence> activeUsers;
  final bool isPresenceLoading;
  final String? lastError;
  final String? connectedUrl;
  final bool isLoadedFromCache;

  const ChatState({
    this.status = SocketStatus.disconnected,
    this.messages = const [],
    this.activeUsers = const [],
    this.isPresenceLoading = true,
    this.lastError,
    this.connectedUrl,
    this.isLoadedFromCache = false,
  });

  bool get isConnected => status == SocketStatus.connected;
  bool get isConnecting => status == SocketStatus.connecting;

  ChatState copyWith({
    SocketStatus? status,
    List<ChatMessage>? messages,
    List<UserPresence>? activeUsers,
    bool? isPresenceLoading,
    String? lastError,
    String? connectedUrl,
    bool? isLoadedFromCache,
  }) {
    return ChatState(
      status: status ?? this.status,
      messages: messages ?? this.messages,
      activeUsers: activeUsers ?? this.activeUsers,
      isPresenceLoading: isPresenceLoading ?? this.isPresenceLoading,
      lastError: lastError,
      connectedUrl: connectedUrl ?? this.connectedUrl,
      isLoadedFromCache: isLoadedFromCache ?? this.isLoadedFromCache,
    );
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is ChatState &&
          runtimeType == other.runtimeType &&
          status == other.status &&
          lastError == other.lastError &&
          connectedUrl == other.connectedUrl &&
          isLoadedFromCache == other.isLoadedFromCache &&
          listEquals(messages, other.messages) &&
          listEquals(activeUsers, other.activeUsers) &&
          isPresenceLoading == other.isPresenceLoading;

  @override
  int get hashCode =>
      status.hashCode ^
      Object.hashAll(messages) ^
      Object.hashAll(activeUsers) ^
      isPresenceLoading.hashCode ^
      lastError.hashCode ^
      connectedUrl.hashCode ^
      isLoadedFromCache.hashCode;
}

// =============================================================================
// Chat Bloc & Cache-to-Live Handoff
// =============================================================================

/// Manages the real-time chat timeline, local offline caching, and WebSocket streams.
///
/// Cache-to-Live Handoff Mechanics:
/// 1. Instant Offline Launch: On app launch or initialization ([ChatInitializeRequested] /
///    [ChatConnectRequested]), cached messages are immediately read from [LocalMessageRepository]
///    and emitted to the UI state. Users see their recent chat history with zero network delay.
/// 2. Live WebSocket Hydration: In the background, the WebSocket connection is established.
///    The server streams the 50 most recent messages.
/// 3. Conflict Resolution: Inbound messages are deduplicated against existing cached items using
///    their unique PostgreSQL UUID ([ChatMessage.id]). The combined timeline is re-sorted
///    chronologically by [ChatMessage.createdAt] ascending.
/// 4. Background Sync: All incoming live and historical messages are written asynchronously to
///    the local database ([LocalMessageRepository.saveMessage]) without blocking the UI thread.
/// Shows an OS notification for a chat message. Injectable for tests.
typedef MessageNotifier = Future<void> Function(ChatMessage message);

Future<void> _defaultNotifier(ChatMessage message) =>
    NotificationService().showMessageNotification(message);

class ChatBloc extends Bloc<ChatEvent, ChatState> {
  /// [tokenProvider] supplies a fresh access token for every (re)connect and sync.
  /// [currentSubProvider] identifies the signed-in user so their own messages
  /// don't raise notifications.
  factory ChatBloc({
    required ChatWebSocketService socketService,
    LocalMessageRepository? localRepo,
    SyncService? syncService,
    String? Function()? tokenProvider,
    String? Function()? currentSubProvider,
    MessageNotifier? notifier,
  }) {
    final repo = localRepo ?? HiveLocalMessageRepository();
    final sync = syncService ??
        SyncService(socketService: socketService, localRepo: repo);
    if (tokenProvider != null) {
      socketService.tokenProvider = tokenProvider;
      sync.tokenProvider = tokenProvider;
    }
    return ChatBloc._(
      socketService,
      repo,
      sync,
      currentSubProvider,
      notifier ?? _defaultNotifier,
    );
  }

  final ChatWebSocketService _socketService;
  final LocalMessageRepository _localRepo;
  final SyncService _syncService;
  final String? Function()? _currentSub;
  final MessageNotifier _notify;
  StreamSubscription<ChatMessage>? _messageSub;
  StreamSubscription<SocketStatus>? _statusSub;
  StreamSubscription<List<ChatMessage>>? _syncSub;
  bool _initialized = false;

  ChatBloc._(
    this._socketService,
    this._localRepo,
    this._syncService,
    this._currentSub,
    this._notify,
  ) : super(const ChatState()) {
    on<ChatInitializeRequested>(_onInitializeRequested);
    on<ChatConnectRequested>(_onConnectRequested);
    on<ChatMessageReceived>(_onMessageReceived);
    on<SyncCompleted>(_onSyncCompleted);
    on<ChatSendMessage>(_onSendMessage);
    on<ChatStatusChanged>(_onStatusChanged);
    on<ChatDisconnectRequested>(_onDisconnectRequested);

    _syncSub = _syncService.onSyncCompleted.listen((messages) {
      add(SyncCompleted(messages));
    });
  }

  Future<void> _onInitializeRequested(
    ChatInitializeRequested event,
    Emitter<ChatState> emit,
  ) async {
    await _performCacheToLiveHandoff(
      accessToken: event.accessToken,
      customWsUrl: event.customWsUrl,
      emit: emit,
    );
  }

  Future<void> _onConnectRequested(
    ChatConnectRequested event,
    Emitter<ChatState> emit,
  ) async {
    if (!_initialized) {
      await _performCacheToLiveHandoff(
        accessToken: event.accessToken,
        customWsUrl: event.customWsUrl,
        emit: emit,
      );
      return;
    }
    // Manual reconnect / token refreshed: reopen the socket with a fresh token.
    _socketService.connect(
      accessToken: event.accessToken,
      customWsUrl: event.customWsUrl,
    );
    _socketService.reconnectNow();
  }

  /// Executes the two-stage cache-to-live handoff pipeline.
  Future<void> _performCacheToLiveHandoff({
    String? accessToken,
    String? customWsUrl,
    required Emitter<ChatState> emit,
  }) async {
    // Idempotent: ResponsiveLayout and ChatScreen may both request initialization,
    // and the layout rebuilds ChatScreen when crossing the 800 px breakpoint.
    if (_initialized) return;
    _initialized = true;

    // -------------------------------------------------------------------------
    // Stage 1: Instant Local Cache Rendering
    // Immediately load persisted messages from local storage (Hive/IndexedDB).
    // -------------------------------------------------------------------------
    try {
      final cached = await _localRepo.getCachedMessages();
      if (cached.isNotEmpty) {
        emit(state.copyWith(
          messages: cached,
          isLoadedFromCache: true,
        ));
      }
    } catch (_) {
      // Gracefully continue to live connection if local cache read encounters an error
    }

    // -------------------------------------------------------------------------
    // Stage 2: Background WebSocket Connection & Hydration
    // -------------------------------------------------------------------------
    await _messageSub?.cancel();
    await _statusSub?.cancel();

    _messageSub = _socketService.messageStream.listen((message) {
      add(ChatMessageReceived(message));
    });

    _statusSub = _socketService.statusStream.listen((status) {
      add(ChatStatusChanged(status, _socketService.lastError));
    });

    emit(state.copyWith(
      status: SocketStatus.connecting,
      connectedUrl: _socketService.connectedUrl,
    ));

    // Configure sync service credentials and snapshot the catch-up cursor
    // *before* the history burst starts writing into the same cache.
    _syncService.setAccessToken(accessToken);
    await _syncService.captureCursor();

    _socketService.connect(
      accessToken: accessToken,
      customWsUrl: customWsUrl,
    );
  }

  Future<void> _onMessageReceived(
    ChatMessageReceived event,
    Emitter<ChatState> emit,
  ) async {
    final incoming = event.message;

    // -------------------------------------------------------------------------
    // Presence Roster Updates:
    // Update active connected users without persisting to message history
    // -------------------------------------------------------------------------
    if (incoming.isPresence) {
      emit(state.copyWith(
        activeUsers: incoming.users,
        isPresenceLoading: false,
      ));
      return;
    }

    // -------------------------------------------------------------------------
    // Synchronization & Deduplication Logic:
    // If incoming message has a PostgreSQL UUID, check if it already exists
    // (e.g., from local cache or prior broadcast).
    // -------------------------------------------------------------------------
    if (incoming.id != null && incoming.id!.isNotEmpty) {
      final exists = state.messages.any((m) => m.id == incoming.id);
      if (exists) {
        // Persist to local cache in case timestamps or attributes were updated
        unawaited(_localRepo.saveMessage(incoming));
        return;
      }
    }

    // Append and maintain ascending chronological order across cached + live messages
    final updatedList = List<ChatMessage>.from(state.messages)..add(incoming);
    updatedList.sort((a, b) => a.createdAt.compareTo(b.createdAt));

    // Notify only for live chat from other people (never for the history burst,
    // join/leave notices or errors). NotificationService itself suppresses
    // notifications while the app is in the foreground.
    final mySub = _currentSub?.call();
    if (incoming.type == 'chat' &&
        incoming.id != null &&
        (mySub == null || incoming.senderId != mySub)) {
      unawaited(_notify(incoming).catchError((Object _) {}));
    }

    emit(state.copyWith(
      messages: updatedList,
      connectedUrl: _socketService.connectedUrl,
    ));

    // Asynchronously save to local database in background
    unawaited(_localRepo.saveMessage(incoming));
  }

  Future<void> _onSyncCompleted(
    SyncCompleted event,
    Emitter<ChatState> emit,
  ) async {
    if (event.messages.isEmpty) return;

    // -------------------------------------------------------------------------
    // Deduplication & Timeline Merge (UST-1.4.3):
    // Compare incoming catch-up messages against the current timeline using PostgreSQL UUID.
    // Filter out any messages already delivered by the live WebSocket 50-message burst
    // or existing local cache.
    // -------------------------------------------------------------------------
    final existingIds = <String>{};
    for (final m in state.messages) {
      if (m.id != null && m.id!.isNotEmpty) {
        existingIds.add(m.id!);
      }
    }

    final newMessages = <ChatMessage>[];
    for (final incoming in event.messages) {
      if (incoming.id != null && incoming.id!.isNotEmpty) {
        if (!existingIds.contains(incoming.id)) {
          existingIds.add(incoming.id!);
          newMessages.add(incoming);
        }
      } else {
        final duplicate = state.messages.any(
          (m) =>
              m.createdAt.isAtSameMomentAs(incoming.createdAt) &&
              m.textContent == incoming.textContent,
        );
        if (!duplicate) {
          newMessages.add(incoming);
        }
      }
    }

    if (newMessages.isEmpty) {
      return;
    }

    // Merge and enforce ascending chronological sorting
    final merged = List<ChatMessage>.from(state.messages)..addAll(newMessages);
    merged.sort((a, b) => a.createdAt.compareTo(b.createdAt));

    emit(state.copyWith(messages: merged));

    // Batch persist newly integrated catch-up messages to local storage
    unawaited(_localRepo.saveMessages(newMessages));
  }

  void _onSendMessage(
    ChatSendMessage event,
    Emitter<ChatState> emit,
  ) {
    if (event.text.trim().isEmpty) return;
    _socketService.sendMessage(event.text.trim());
  }

  void _onStatusChanged(
    ChatStatusChanged event,
    Emitter<ChatState> emit,
  ) {
    final isConnected = event.status == SocketStatus.connected;
    emit(state.copyWith(
      status: event.status,
      lastError: event.error,
      connectedUrl: _socketService.connectedUrl,
      activeUsers: isConnected ? state.activeUsers : const [],
      isPresenceLoading: !isConnected,
    ));
  }

  void _onDisconnectRequested(
    ChatDisconnectRequested event,
    Emitter<ChatState> emit,
  ) {
    _socketService.disconnect();
    emit(state.copyWith(
      status: SocketStatus.disconnected,
      activeUsers: const [],
      isPresenceLoading: true,
    ));
  }

  @override
  Future<void> close() async {
    await _messageSub?.cancel();
    await _statusSub?.cancel();
    await _syncSub?.cancel();
    _syncService.dispose();
    return super.close();
  }
}
