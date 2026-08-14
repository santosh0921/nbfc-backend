import 'package:flutter/material.dart';
import '../../core/services/notification_service.dart';
import '../../core/services/permission_service.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';

/// Real Android permission requests for Notifications and Location —
/// tapping "Enable" triggers the actual OS system permission dialog via
/// permission_handler, not a mock toggle. A test notification button
/// confirms flutter_local_notifications is genuinely delivering to the
/// device's notification tray once notification access is granted.
class AppPermissionsScreen extends StatefulWidget {
  const AppPermissionsScreen({super.key});

  @override
  State<AppPermissionsScreen> createState() => _AppPermissionsScreenState();
}

class _AppPermissionsScreenState extends State<AppPermissionsScreen> with WidgetsBindingObserver {
  bool? _notificationGranted;
  bool? _locationGranted;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _refreshStatus();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // Re-check when the user comes back from the system Settings screen.
    if (state == AppLifecycleState.resumed) _refreshStatus();
  }

  Future<void> _refreshStatus() async {
    final notif = await PermissionService.isNotificationGranted();
    final loc = await PermissionService.isLocationGranted();
    if (!mounted) return;
    setState(() {
      _notificationGranted = notif;
      _locationGranted = loc;
    });
  }

  Future<void> _requestNotification() async {
    final granted = await PermissionService.requestNotification();
    if (!mounted) return;
    setState(() => _notificationGranted = granted);
    if (!granted) _showSettingsPrompt('Notifications');
  }

  Future<void> _requestLocation() async {
    final granted = await PermissionService.requestLocation();
    if (!mounted) return;
    setState(() => _locationGranted = granted);
    if (!granted) _showSettingsPrompt('Location');
  }

  void _showSettingsPrompt(String label) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('$label permission was denied. Enable it from system Settings.'),
        action: const SnackBarAction(label: 'Settings', onPressed: PermissionService.openSettings),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('App Permissions')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Text(
              'Grant these permissions for the full NBFC Premium experience.',
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 20),
            _PermissionCard(
              icon: Icons.notifications_active_rounded,
              title: 'Notifications',
              description: 'Get real-time alerts for EMI due dates, application status, and payment confirmations.',
              granted: _notificationGranted,
              onRequest: _requestNotification,
            ),
            const SizedBox(height: 14),
            _PermissionCard(
              icon: Icons.location_on_rounded,
              title: 'Location',
              description: 'Find the nearest branch and get location-based loan offers relevant to you.',
              granted: _locationGranted,
              onRequest: _requestLocation,
            ),
            const SizedBox(height: 24),
            if (_notificationGranted == true)
              OutlinedButton.icon(
                onPressed: () => NotificationService.instance.show(
                  title: 'NBFC Premium',
                  body: 'Notifications are working — you\'ll get real alerts like this one.',
                ),
                icon: const Icon(Icons.send_rounded, size: 18),
                label: const Text('Send Test Notification'),
                style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
              ),
          ],
        ),
      ),
    );
  }
}

class _PermissionCard extends StatelessWidget {
  const _PermissionCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.granted,
    required this.onRequest,
  });

  final IconData icon;
  final String title;
  final String description;
  final bool? granted;
  final VoidCallback onRequest;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isGranted = granted == true;
    return PremiumCard(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: (isGranted ? AppColors.success : AppColors.primary).withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(icon, color: isGranted ? AppColors.success : AppColors.primary),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: theme.textTheme.titleSmall),
                const SizedBox(height: 4),
                Text(description, style: theme.textTheme.bodySmall),
                const SizedBox(height: 10),
                if (isGranted)
                  Row(
                    children: [
                      const Icon(Icons.check_circle_rounded, size: 15, color: AppColors.success),
                      const SizedBox(width: 6),
                      Text('Granted', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success, fontWeight: FontWeight.w700)),
                    ],
                  )
                else
                  SizedBox(
                    height: 36,
                    child: OutlinedButton(
                      onPressed: onRequest,
                      style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 16)),
                      child: const Text('Enable'),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
