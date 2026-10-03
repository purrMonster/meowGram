import 'dart:async';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';

// =============================================================================
// Chat Events
// =============================================================================

abstract class ChatEvent {
  const ChatEvent();
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

  const ChatState({
    this.status = SocketStatus.disconnected,
    this.messages = const [],
    this.lastError,
    this.connectedUrl,
  });

  bool get isConnected => status == SocketStatus.connected;
  bool get isConnecting => status == SocketStatus.connecting;

  ChatState copyWith({
    SocketStatus? status,
    List<ChatMessage>? messages,
    String? lastError,
    String? connectedUrl,
  }) {
    return ChatState(
      status: status ?? this.status,
      messages: messages ?? this.messages,
      lastError: lastError,
      connectedUrl: connectedUrl ?? this.connectedUrl,
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
          messages.length == other.messages.length;

  @override
  int get hashCode =>
      status.hashCode ^
      messages.hashCode ^
      lastError.hashCode ^
      connectedUrl.hashCode;
}

// =============================================================================
// Chat Bloc
// =============================================================================

/// Manages the real-time chat timeline, historical message batching, and WebSocket stream orchestration.
class ChatBloc extends Bloc<ChatEvent, ChatState> {
  final ChatWebSocketService _socketService;
  StreamSubscription<ChatMessage>? _messageSub;
  StreamSubscription<SocketStatus>? _statusSub;

  ChatBloc({required ChatWebSocketService socketService})
      : _socketService = socketService,
        super(const ChatState()) {
    on<ChatConnectRequested>(_onConnectRequested);
    on<ChatMessageReceived>(_onMessageReceived);
    on<ChatSendMessage>(_onSendMessage);
    on<ChatStatusChanged>(_onStatusChanged);
    on<ChatDisconnectRequested>(_onDisconnectRequested);
  }

  Future<void> _onConnectRequested(
    ChatConnectRequested event,
    Emitter<ChatState> emit,
  ) async {
    // Cancel any previous subscriptions
    await _messageSub?.cancel();
    await _statusSub?.cancel();

    // Listen to new incoming messages (history burst and real-time broadcasts)
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
      accessToken: event.accessToken,
      customWsUrl: event.customWsUrl,
    );
  }

  void _onMessageReceived(
    ChatMessageReceived event,
    Emitter<ChatState> emit,
  ) {
    final incoming = event.message;

    // Deduplicate if message has a PostgreSQL UUID and is already present
    if (incoming.id != null && incoming.id!.isNotEmpty) {
      final exists = state.messages.any((m) => m.id == incoming.id);
      if (exists) {
        return;
      }
    }

    // Append and maintain ascending chronological order
    final updatedList = List<ChatMessage>.from(state.messages)..add(incoming);
    updatedList.sort((a, b) => a.createdAt.compareTo(b.createdAt));

    emit(state.copyWith(
      messages: updatedList,
      connectedUrl: _socketService.connectedUrl,
    ));
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
