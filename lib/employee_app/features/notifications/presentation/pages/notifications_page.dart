import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../domain/entities/notification_entity.dart';
import '../providers/notifications_provider.dart';

class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key});

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<NotificationsProvider>().load());
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<NotificationsProvider>();

    return AppScaffold(
      title: 'Notifications',
      subtitle: provider.unreadCount > 0 ? '${provider.unreadCount} unread' : 'All caught up',
      actions: [
        if (provider.unreadCount > 0)
          TextButton(
            onPressed: () => context.read<NotificationsProvider>().markAllAsRead(),
            child: const Text('Mark all read'),
          ),
      ],
      padding: const EdgeInsets.all(AppSpacing.md),
      body: provider.isLoading
          ? const Center(child: CircularProgressIndicator())
          : provider.items.isEmpty
              ? Center(child: Text('No notifications yet', style: Theme.of(context).textTheme.bodyMedium))
              : RefreshIndicator(
                  onRefresh: provider.load,
                  child: ListView.separated(
                    physics: const AlwaysScrollableScrollPhysics(),
                    itemCount: provider.items.length,
                    separatorBuilder: (_, __) => const SizedBox(height: AppSpacing.sm),
                    itemBuilder: (context, index) {
                      final item = provider.items[index];
                      return _NotificationTile(
                        notification: item,
                        onTap: () {
                          if (!item.isRead) context.read<NotificationsProvider>().markAsRead(item.id);
                        },
                      );
                    },
                  ),
                ),
    );
  }
}

class _NotificationTile extends StatelessWidget {
  const _NotificationTile({required this.notification, required this.onTap});

  final NotificationEntity notification;
  final VoidCallback onTap;

  IconData get _icon => switch (notification.type) {
        NotificationType.newAssignment => Icons.assignment_add,
        NotificationType.visitReminder => Icons.schedule_outlined,
        NotificationType.caseUpdated => Icons.sync_outlined,
        NotificationType.reportAccepted => Icons.check_circle_outline,
        NotificationType.reportRejected => Icons.cancel_outlined,
      };

  Color get _color => switch (notification.type) {
        NotificationType.newAssignment => AppColors.info,
        NotificationType.visitReminder => AppColors.warning,
        NotificationType.caseUpdated => AppColors.primary,
        NotificationType.reportAccepted => AppColors.success,
        NotificationType.reportRejected => AppColors.danger,
      };

  String _timeAgo() {
    final diff = DateTime.now().difference(notification.timestamp);
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    return '${diff.inDays}d ago';
  }

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: onTap,
      withShadow: !notification.isRead,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 40,
            height: 40,
            alignment: Alignment.center,
            decoration: BoxDecoration(color: _color.withValues(alpha: 0.12), shape: BoxShape.circle),
            child: Icon(_icon, color: _color, size: 20),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        notification.title,
                        style: Theme.of(context).textTheme.titleSmall?.copyWith(
                              fontWeight: notification.isRead ? FontWeight.w500 : FontWeight.w700,
                            ),
                      ),
                    ),
                    if (!notification.isRead)
                      Container(
                        width: 8,
                        height: 8,
                        decoration: const BoxDecoration(color: AppColors.gold, shape: BoxShape.circle),
                      ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(notification.message, style: Theme.of(context).textTheme.bodySmall),
                const SizedBox(height: 6),
                Text(_timeAgo(), style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
