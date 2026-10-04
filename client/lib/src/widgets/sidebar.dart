import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:meowgram_client/src/auth/auth_controller.dart';
import 'package:meowgram_client/src/bloc/chat_bloc.dart';
import 'package:meowgram_client/src/models/chat_message.dart';
import 'package:meowgram_client/src/services/chat_websocket_service.dart';
import 'package:meowgram_client/src/widgets/connection_badge.dart';

/// Desktop & Tablet sidebar displaying chat rooms, active members, and user profile status.
class Sidebar extends StatelessWidget {
  final AuthController authController;
  final String selectedRoom;
  final ValueChanged<String>? onRoomSelected;
  final VoidCallback? onLogoutPressed;
  final List<UserPresence>? activeUsers;
  final bool? isPresenceLoading;

  const Sidebar({
    super.key,
    required this.authController,
    this.selectedRoom = 'general-lounge',
    this.onRoomSelected,
    this.onLogoutPressed,
    this.activeUsers,
    this.isPresenceLoading,
  });

  static const List<Map<String, dynamic>> _rooms = [
    {'id': 'general-lounge', 'name': 'general-lounge', 'icon': Icons.pets_rounded, 'unread': 0},
    {'id': 'cat-memes', 'name': 'cat-memes', 'icon': Icons.image_outlined, 'unread': 3},
    {'id': 'paw-sitive-vibes', 'name': 'paw-sitive-vibes', 'icon': Icons.favorite_outline_rounded, 'unread': 0},
    {'id': 'treat-discussions', 'name': 'treat-discussions', 'icon': Icons.restaurant_menu_rounded, 'unread': 1},
    {'id': 'yarn-and-toys', 'name': 'yarn-and-toys', 'icon': Icons.sports_tennis_rounded, 'unread': 0},
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final profile = authController.userProfile;
    final username = profile?.username ?? 'Whiskers';
    final sub = profile?.sub ?? '';

    // Dynamically retrieve presence roster from ChatBloc if available
    ChatState? chatState;
    try {
      chatState = context.watch<ChatBloc>().state;
    } catch (_) {}

    final presenceList = activeUsers ?? chatState?.activeUsers ?? const <UserPresence>[];
    final isLoading = isPresenceLoading ?? (chatState == null ? false : chatState.isPresenceLoading);
    final onlineCount = presenceList.where((u) => u.isOnline).length;

    return Container(
      width: 270,
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLow,
        border: Border(
          right: BorderSide(
            color: theme.colorScheme.outlineVariant.withOpacity(0.4),
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 1. Lounge Brand Header
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 16.0),
            decoration: BoxDecoration(
              border: Border(
                bottom: BorderSide(
                  color: theme.colorScheme.outlineVariant.withOpacity(0.3),
                ),
              ),
            ),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(8.0),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primaryContainer,
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    Icons.pets_rounded,
                    size: 20,
                    color: theme.colorScheme.onPrimaryContainer,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'meowGram',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                          letterSpacing: 0.5,
                        ),
                      ),
                      Text(
                        'Lounge Rooms',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ),
                ConnectionBadge(status: chatState?.status ?? SocketStatus.disconnected),
              ],
            ),
          ),

          // 2. Chat Rooms Section
          Expanded(
            child: ListView(
              padding: const EdgeInsets.symmetric(vertical: 8.0),
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 6.0),
                  child: Text(
                    'CHANNELS',
                    style: theme.textTheme.labelSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                      color: theme.colorScheme.onSurfaceVariant.withOpacity(0.7),
                      letterSpacing: 1.0,
                    ),
                  ),
                ),
                for (final room in _rooms)
                  _buildRoomTile(
                    context: context,
                    id: room['id'] as String,
                    name: room['name'] as String,
                    icon: room['icon'] as IconData,
                    unread: room['unread'] as int,
                    isSelected: selectedRoom == room['id'],
                  ),

                const SizedBox(height: 16),

                // 3. Active Lounge Members Section
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 6.0),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Flexible(
                        child: Text(
                          'LOUNGE MEMBERS',
                          style: theme.textTheme.labelSmall?.copyWith(
                            fontWeight: FontWeight.bold,
                            color: theme.colorScheme.onSurfaceVariant.withOpacity(0.7),
                            letterSpacing: 1.0,
                          ),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: onlineCount > 0 ? Colors.green.withOpacity(0.15) : Colors.grey.withOpacity(0.15),
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: Text(
                          '$onlineCount online',
                          style: TextStyle(
                            fontSize: 10,
                            color: onlineCount > 0 ? Colors.green : Colors.grey,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                if (isLoading && presenceList.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 10.0),
                    child: Row(
                      children: [
                        Icon(
                          Icons.sync_rounded,
                          size: 14,
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            'Connecting to lounge...',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                              fontSize: 11,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                  )
                else if (presenceList.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 12.0),
                    child: Text(
                      'No other cats online yet',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                        fontSize: 11,
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  )
                else
                  for (final user in presenceList)
                    _buildUserTile(
                      context: context,
                      name: user.username,
                      statusText: user.sub == sub
                          ? 'You (Purring)'
                          : (user.isOnline ? 'Online in lounge' : 'Away'),
                      isOnline: user.isOnline,
                    ),
              ],
            ),
          ),

          // 4. Authenticated User Profile Footer
          Container(
            padding: const EdgeInsets.all(12.0),
            decoration: BoxDecoration(
              color: theme.colorScheme.surfaceContainer,
              border: Border(
                top: BorderSide(
                  color: theme.colorScheme.outlineVariant.withOpacity(0.4),
                ),
              ),
            ),
            child: Row(
              children: [
                CircleAvatar(
                  radius: 18,
                  backgroundColor: theme.colorScheme.primary,
                  child: Text(
                    username.isNotEmpty ? username[0].toUpperCase() : 'C',
                    style: TextStyle(
                      color: theme.colorScheme.onPrimary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        '@$username',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                      Text(
                        sub.isNotEmpty ? sub : 'OIDC Verified',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                          fontSize: 10,
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.logout_rounded, size: 20),
                  tooltip: 'Logout session',
                  color: theme.colorScheme.error,
                  onPressed: onLogoutPressed ?? () => authController.logout(),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildRoomTile({
    required BuildContext context,
    required String id,
    required String name,
    required IconData icon,
    required int unread,
    required bool isSelected,
  }) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 8.0, vertical: 2.0),
      child: Material(
        color: Colors.transparent,
        child: ListTile(
          dense: true,
          selected: isSelected,
          selectedTileColor: theme.colorScheme.primaryContainer.withOpacity(0.5),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(10.0),
          ),
          leading: Icon(
            icon,
            size: 18,
            color: isSelected ? theme.colorScheme.primary : theme.colorScheme.onSurfaceVariant,
          ),
          title: Text(
            '# $name',
            style: TextStyle(
              fontSize: 13,
              fontWeight: isSelected ? FontWeight.bold : FontWeight.w500,
              color: isSelected ? theme.colorScheme.primary : theme.colorScheme.onSurface,
            ),
          ),
          trailing: unread > 0
              ? Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primary,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    '$unread',
                    style: TextStyle(
                      fontSize: 10,
                      color: theme.colorScheme.onPrimary,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                )
              : null,
          onTap: () => onRoomSelected?.call(id),
        ),
      ),
    );
  }

  Widget _buildUserTile({
    required BuildContext context,
    required String name,
    required String statusText,
    required bool isOnline,
  }) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12.0, vertical: 3.0),
      child: Row(
        children: [
          Stack(
            children: [
              CircleAvatar(
                radius: 13,
                backgroundColor: theme.colorScheme.surfaceContainerHighest,
                child: Text(
                  name.isNotEmpty ? name[0].toUpperCase() : '?',
                  style: const TextStyle(fontSize: 11, fontWeight: FontWeight.bold),
                ),
              ),
              Positioned(
                bottom: 0,
                right: 0,
                child: Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(
                    color: isOnline ? Colors.green : Colors.grey,
                    shape: BoxShape.circle,
                    border: Border.all(
                      color: theme.colorScheme.surface,
                      width: 1.5,
                    ),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
                ),
                Text(
                  statusText,
                  style: TextStyle(
                    fontSize: 10,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
