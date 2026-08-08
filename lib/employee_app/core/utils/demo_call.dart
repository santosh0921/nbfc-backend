import 'package:flutter/material.dart';
import '../components/app_card.dart';
import '../components/app_snackbar.dart';
import '../constants/app_colors.dart';
import '../constants/app_spacing.dart';

/// Simulated call UI — this build never launches the real Android dialer.
/// A real `tel:` intent backgrounds the Flutter app and, on some OEMs
/// (aggressive background-process killing), can tear down the whole engine
/// and leave the app blank on return. The demo flow avoids that entirely
/// while still giving the field officer a believable "call in progress"
/// moment before logging the outcome.
Future<void> showDemoCall(BuildContext context,
    {required String name, required String phone}) async {
  await showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (context) => _DemoCallDialog(name: name, phone: phone),
  );
}

/// Full call workflow, reused everywhere a "Call" button appears (Case
/// Detail, dashboard task cards, ...): plays the demo call, then always
/// asks the officer to log what the customer said before doing anything
/// else. On "Yes, agreed to proceed" it invokes [onProceed] — typically
/// starting the verification flow for that customer.
Future<void> showCallOutcomeFlow(
  BuildContext context, {
  required String name,
  required String phone,
  required VoidCallback onProceed,
}) async {
  await showDemoCall(context, name: name, phone: phone);
  if (!context.mounted) return;

  final outcome = await showModalBottomSheet<String>(
    context: context,
    isScrollControlled: true,
    builder: (context) => Padding(
      padding: const EdgeInsets.all(AppSpacing.lg),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.only(bottom: AppSpacing.md),
                decoration: BoxDecoration(color: AppColors.border, borderRadius: BorderRadius.circular(AppSpacing.radiusFull)),
              ),
            ),
            Text('What did the customer say?', style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: 4),
            Text(
              'Log the call outcome for $name before continuing.',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
            ),
            const SizedBox(height: AppSpacing.lg),
            _CallOutcomeOption(
              icon: Icons.check_circle_outline,
              color: AppColors.success,
              title: 'Yes, agreed to proceed',
              subtitle: 'Start the field verification visit now',
              onTap: () => Navigator.of(context).pop('proceed'),
            ),
            const SizedBox(height: AppSpacing.sm),
            _CallOutcomeOption(
              icon: Icons.schedule_outlined,
              color: AppColors.warning,
              title: 'Asked to reschedule',
              subtitle: 'Customer wants a different date/time',
              onTap: () => Navigator.of(context).pop('reschedule'),
            ),
            const SizedBox(height: AppSpacing.sm),
            _CallOutcomeOption(
              icon: Icons.cancel_outlined,
              color: AppColors.danger,
              title: 'Declined / Not reachable',
              subtitle: 'Customer refused or call did not connect',
              onTap: () => Navigator.of(context).pop('declined'),
            ),
          ],
        ),
      ),
    ),
  );

  if (!context.mounted || outcome == null) return;

  switch (outcome) {
    case 'proceed':
      onProceed();
    case 'reschedule':
      AppSnackbar.info(context, 'Visit marked for rescheduling.');
    case 'declined':
      AppSnackbar.danger(context, 'Call outcome logged as declined.');
  }
}

class _CallOutcomeOption extends StatelessWidget {
  const _CallOutcomeOption({required this.icon, required this.color, required this.title, required this.subtitle, required this.onTap});

  final IconData icon;
  final Color color;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: onTap,
      child: Row(
        children: [
          Container(
            width: 42,
            height: 42,
            alignment: Alignment.center,
            decoration: BoxDecoration(color: color.withValues(alpha: 0.12), shape: BoxShape.circle),
            child: Icon(icon, color: color, size: 22),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: Theme.of(context).textTheme.titleSmall),
                Text(subtitle, style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
              ],
            ),
          ),
          const Icon(Icons.chevron_right),
        ],
      ),
    );
  }
}

class _DemoCallDialog extends StatefulWidget {
  const _DemoCallDialog({required this.name, required this.phone});

  final String name;
  final String phone;

  @override
  State<_DemoCallDialog> createState() => _DemoCallDialogState();
}

class _DemoCallDialogState extends State<_DemoCallDialog>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pulseController = AnimationController(
      vsync: this, duration: const Duration(milliseconds: 1200))
    ..repeat(reverse: true);

  bool _popped = false;

  @override
  void initState() {
    super.initState();
    Future.delayed(const Duration(milliseconds: 2200), _popOnce);
  }

  // Guards against a double-pop: without this, the auto-dismiss timer and
  // a system/gesture back-press racing each other could both call
  // Navigator.pop() for this route, popping an extra (unrelated) route
  // underneath and corrupting the navigation stack — the app would then
  // sit on an invisible leftover barrier that silently eats every tap.
  void _popOnce() {
    if (_popped || !mounted) return;
    _popped = true;
    Navigator.of(context).pop();
  }

  @override
  void dispose() {
    _pulseController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      // The call can't be manually backed out of — it only ever closes via
      // the timer above, so there's exactly one path that can pop this
      // route.
      canPop: false,
      child: Dialog(
        shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppSpacing.radiusLg)),
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ScaleTransition(
                scale: Tween(begin: 0.92, end: 1.08).animate(CurvedAnimation(
                    parent: _pulseController, curve: Curves.easeInOut)),
                child: Container(
                  width: 76,
                  height: 76,
                  alignment: Alignment.center,
                  decoration: const BoxDecoration(
                      color: AppColors.success, shape: BoxShape.circle),
                  child: const Icon(Icons.call, color: Colors.white, size: 32),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('Calling…',
                  style: Theme.of(context)
                      .textTheme
                      .bodyMedium
                      ?.copyWith(color: AppColors.textSecondary)),
              const SizedBox(height: 4),
              Text(widget.name, style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 2),
              Text(widget.phone,
                  style: Theme.of(context)
                      .textTheme
                      .bodyMedium
                      ?.copyWith(color: AppColors.textSecondary)),
              const SizedBox(height: AppSpacing.md),
              Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.sm, vertical: 4),
                decoration: BoxDecoration(
                    color: AppColors.primary.withValues(alpha: 0.14),
                    borderRadius: BorderRadius.circular(AppSpacing.radiusFull)),
                child: Text('Demo mode',
                    style: Theme.of(context)
                        .textTheme
                        .labelSmall
                        ?.copyWith(color: AppColors.secondary)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
