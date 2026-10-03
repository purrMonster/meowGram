import 'dart:async';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/storage/local_message_repository.dart';

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
  final String? lastError;
  final String? connectedUrl;
  final bool isLoadedFromCache;

  const ChatState({
    this.status = SocketStatus.disconnected,
    this.messages = const [],
    this.lastError,
    this.connectedUrl,
    this.isLoadedFromCache = false,
  });

  bool get isConnected => status == SocketStatus.connected;
  bool get isConnecting => status == SocketStatus.connecting;

  ChatState copyWith({
    SocketStatus? status,
    List<ChatMessage>? messages,
    String? lastError,
    String? connectedUrl,
    bool? isLoadedFromCache,
  }) {
    return ChatState(
      status: status ?? this.status,
      messages: messages ?? this.messages,
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
          messages.length == other.messages.length;

  @override
  int get hashCode =>
      status.hashCode ^
      messages.hashCode ^
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
class ChatBloc extends Bloc<ChatEvent, ChatState> {
  final ChatWebSocketService _socketService;
  final LocalMessageRepository _localRepo;
  StreamSubscription<ChatMessage>? _messageSub;
  StreamSubscription<SocketStatus>? _statusSub;

  ChatBloc({
    required ChatWebSocketService socketService,
    LocalMessageRepository? localRepo,
  })  : _socketService = socketService,
        _localRepo = localRepo ?? HiveLocalMessageRepository(),
        super(const ChatState()) {
    on<ChatInitializeRequested>(_onInitializeRequested);
    on<ChatConnectRequested>(_onConnectRequested);
    on<ChatMessageReceived>(_onMessageReceived);
    on<ChatSendMessage>(_onSendMessage);
    on<ChatStatusChanged>(_onStatusChanged);
    on<ChatDisconnectRequested>(_onDisconnectRequested);
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
    await _performCacheToLiveHandoff(
      accessToken: event.accessToken,
      customWsUrl: event.customWsUrl,
      emit: emit,
    );
  }

  /// Executes the two-stage cache-to-live handoff pipeline.
  Future<void> _performCacheToLiveHandoff({
    String? accessToken,
    String? customWsUrl,
    required Emitter<ChatState> emit,
  }) async {
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

    emit(state.copyWith(
      messages: updatedList,
      connectedUrl: _socketService.connectedUrl,
    ));

    // Asynchronously save to local database in background
    unawaited(_localRepo.saveMessage(incoming));
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
    emit(state.copyWith(
      status: event.status,
      lastError: event.error,
      connectedUrl: _socketService.connectedUrl,
    ));
  }

  void _onDisconnectRequested(
    ChatDisconnectRequested event,
    Emitter<ChatState> emit,
  ) {
    _socketService.disconnect();
    emit(state.copyWith(status: SocketStatus.disconnected));
  }

  @override
  Future<void> close() async {
    await _messageSub?.cancel();
    await _statusSub?.cancel();
    return super.close();
  }
}
