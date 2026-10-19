import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';
import '../../../../core/components/app_bottom_sheet.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/utils/demo_call.dart';
import '../../../../../core/widgets/premium_shine.dart';
import '../../../../../core/widgets/auto_refresh_mixin.dart';
import '../../../../core/widgets/app_badge.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../auth/presentation/providers/auth_provider.dart';
import '../../../notifications/presentation/providers/notifications_provider.dart';
import '../../../visit_verification/presentation/pages/verification_flow_page.dart';
import '../../domain/entities/dashboard_summary.dart';
import '../providers/dashboard_provider.dart';
import '../widgets/quick_actions.dart';

class DashboardHomePage extends StatefulWidget {
  const DashboardHomePage({super.key});

  @override
  State<DashboardHomePage> createState() => _DashboardHomePageState();
}

class _DashboardHomePageState extends State<DashboardHomePage> with AutoRefreshMixin {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<DashboardProvider>().load());
    startAutoRefresh();
  }

  @override
  Future<void> onAutoRefresh() async {
    if (!mounted) return;
    await context.read<DashboardProvider>().load(silent: true);
  }

  @override
  void dispose() {
    stopAutoRefresh();
    super.dispose();
  }

  void _openTaskDetail(TaskItem task) {
    AppBottomSheet.show(
      context,
      title: task.title,
      child: _TaskDetailContent(task: task),
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final dashboard = context.watch<DashboardProvider>();
    final unread = context.watch<NotificationsProvider>().unreadCount;
    final employee = auth.employee;

    return AppScaffold(
      showAppBar: false,
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
      drawer: _AppDrawer(name: employee?.name ?? 'Employee', designation: employee?.designation ?? ''),
      body: dashboard.isLoading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: () => context.read<DashboardProvider>().load(),
              child: ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                // Cards now carry a real shadow (see AppCard) — ListView's
                // default Clip.hardEdge clips that shadow off in a flat
                // line right at the list's own edge, which reads as the
                // card's left/right corners being chopped since the
                // horizontal page padding gave the shadow nowhere to
                // bleed into.
                clipBehavior: Clip.none,
                children: [
                  const SizedBox(height: AppSpacing.sm),
                  _TopBar(name: employee?.name ?? 'Employee', unreadCount: unread),
                  const SizedBox(height: AppSpacing.lg),
                  _PromoBanner(designation: employee?.designation ?? 'Field Officer'),
                  const SizedBox(height: AppSpacing.lg),
                  Row(
                    children: [
                      Expanded(
                        child: _TaskStatusCard(
                          label: 'Completed',
                          count: dashboard.completedCount,
                          icon: Icons.check_circle_outline,
                          color: AppColors.success,
                          selected: dashboard.filter == TaskStatus.completed,
                          onTap: () => context.read<DashboardProvider>().setFilter(TaskStatus.completed),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: _TaskStatusCard(
                          label: 'Remaining',
                          count: dashboard.remainingCount,
                          icon: Icons.hourglass_empty,
                          color: AppColors.info,
                          selected: dashboard.filter == TaskStatus.remaining,
                          onTap: () => context.read<DashboardProvider>().setFilter(TaskStatus.remaining),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: _TaskStatusCard(
                          label: 'Postponed',
                          count: dashboard.postponedCount,
                          icon: Icons.pause_circle_outline,
                          color: AppColors.warning,
                          selected: dashboard.filter == TaskStatus.postponed,
                          onTap: () => context.read<DashboardProvider>().setFilter(TaskStatus.postponed),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Flexible(
                        child: Text(
                          "Today's Tasks",
                          style: Theme.of(context).textTheme.headlineSmall,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Text(
                        DateFormat('EEE, d MMM').format(DateTime.now()),
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  if (dashboard.filteredTasks.isEmpty)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
                      child: Center(
                        child: Text('No tasks in this category', style: Theme.of(context).textTheme.bodyMedium),
                      ),
                    )
                  else
                    ...dashboard.filteredTasks.map((t) => Padding(
                          padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                          child: _TaskTile(task: t, onTap: () => _openTaskDetail(t)),
                        )),
                  const SizedBox(height: AppSpacing.lg),
                  Text('Quick Actions', style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: AppSpacing.sm),
                  const _QuickActionsGrid(),
                  const SizedBox(height: AppSpacing.xxl),
                ],
              ),
            ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.name, required this.unreadCount});

  final String name;
  final int unreadCount;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        InkWell(
          borderRadius: BorderRadius.circular(AppSpacing.radiusMd),
          onTap: () => Scaffold.of(context).openDrawer(),
          child: Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(color: AppColors.surfaceMuted, borderRadius: BorderRadius.circular(AppSpacing.radiusMd)),
            child: const Icon(Icons.menu_rounded, color: AppColors.textPrimary),
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Hello,', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppColors.textSecondary)),
              Text(
                name,
                style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
        ),
        InkWell(
          borderRadius: BorderRadius.circular(AppSpacing.radiusFull),
          onTap: () => context.goNamed(RouteNames.notifications),
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              Container(
                width: 40,
                height: 40,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: AppColors.surfaceMuted, borderRadius: BorderRadius.circular(AppSpacing.radiusFull)),
                child: const Icon(Icons.notifications_none_rounded, color: AppColors.textPrimary),
              ),
              if (unreadCount > 0)
                Positioned(
                  top: -2,
                  right: -2,
                  child: Container(
                    width: 16,
                    height: 16,
                    alignment: Alignment.center,
                    decoration: const BoxDecoration(color: AppColors.danger, shape: BoxShape.circle),
                    child: Text(
                      unreadCount > 9 ? '9+' : '$unreadCount',
                      style: const TextStyle(color: Colors.white, fontSize: 9, fontWeight: FontWeight.w700),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }
}

class _PromoBanner extends StatelessWidget {
  const _PromoBanner({required this.designation});

  final String designation;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        boxShadow: [BoxShadow(color: AppColors.primaryDark.withValues(alpha: 0.22), blurRadius: 20, offset: const Offset(0, 8))],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: Container(
          decoration: const BoxDecoration(
            gradient: LinearGradient(colors: AppColors.heroGradient, begin: Alignment.topLeft, end: Alignment.bottomRight),
          ),
          child: Stack(
            children: [
              Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            designation.toUpperCase(),
                            style: const TextStyle(color: AppColors.secondary, fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 0.4),
                          ),
                          const SizedBox(height: 6),
                          const Text(
                            'FIELD OPS,\nMADE SIMPLE.',
                            style: TextStyle(color: AppColors.secondary, fontSize: 20, fontWeight: FontWeight.w800, height: 1.15),
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Container(
                            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
                            decoration: BoxDecoration(color: AppColors.secondary, borderRadius: BorderRadius.circular(AppSpacing.radiusFull)),
                            child: const Text('View Cases', style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
                          ),
                        ],
                      ),
                    ),
                    const Icon(Icons.badge_rounded, color: AppColors.secondary, size: 56),
                  ],
                ),
              ),
              const Positioned.fill(child: PremiumShine(opacity: 0.3)),
            ],
          ),
        ),
      ),
    );
  }
}

class _TaskStatusCard extends StatelessWidget {
  const _TaskStatusCard({
    required this.label,
    required this.count,
    required this.icon,
    required this.color,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final int count;
  final IconData icon;
  final Color color;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return AnimatedContainer(
      duration: const Duration(milliseconds: 180),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        border: Border.all(color: selected ? color : AppColors.border, width: selected ? 1.5 : 1),
        color: selected ? color.withValues(alpha: 0.08) : Colors.white,
        boxShadow: [
          BoxShadow(
            color: (selected ? color : AppColors.shadow).withValues(alpha: selected ? 0.18 : 1),
            blurRadius: selected ? 16 : 8,
            offset: const Offset(0, 3),
          ),
        ],
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.md, horizontal: AppSpacing.sm),
          child: Column(
            children: [
              Container(
                width: 32,
                height: 32,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: color.withValues(alpha: 0.14), shape: BoxShape.circle),
                child: Icon(icon, color: color, size: 18),
              ),
              const SizedBox(height: 6),
              FittedBox(
                fit: BoxFit.scaleDown,
                child: Text('$count', style: Theme.of(context).textTheme.titleLarge),
              ),
              Text(
                label,
                style: Theme.of(context).textTheme.bodySmall,
                textAlign: TextAlign.center,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _TaskTile extends StatelessWidget {
  const _TaskTile({required this.task, required this.onTap});

  final TaskItem task;
  final VoidCallback onTap;

  AppBadgeVariant get _priorityVariant => task.priority == 'High' ? AppBadgeVariant.danger : AppBadgeVariant.warning;

  AppBadgeVariant get _statusVariant => switch (task.status) {
        TaskStatus.completed => AppBadgeVariant.success,
        TaskStatus.remaining => AppBadgeVariant.info,
        TaskStatus.postponed => AppBadgeVariant.warning,
      };

  String get _statusLabel => switch (task.status) {
        TaskStatus.completed => 'Completed',
        TaskStatus.remaining => 'Remaining',
        TaskStatus.postponed => 'Postponed',
      };

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: onTap,
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(task.title, style: Theme.of(context).textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 4),
                Text(task.customerName, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 6),
                Wrap(
                  spacing: AppSpacing.sm,
                  runSpacing: 4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    AppBadge(label: task.priority, variant: _priorityVariant),
                    AppBadge(label: _statusLabel, variant: _statusVariant),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.schedule, size: 14, color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.5)),
                        const SizedBox(width: 4),
                        Text(task.time, style: Theme.of(context).textTheme.bodySmall),
                      ],
                    ),
                  ],
                ),
              ],
            ),
          ),
          const Icon(Icons.chevron_right),
        ],
      ),
    );
  }
}

class _TaskDetailContent extends StatelessWidget {
  const _TaskDetailContent({required this.task});

  final TaskItem task;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _DetailRow(icon: Icons.person_outline, label: 'Customer', value: task.customerName),
        _DetailRow(icon: Icons.call_outlined, label: 'Phone', value: task.customerPhone),
        _DetailRow(icon: Icons.event_outlined, label: 'Date', value: DateFormat('EEE, d MMM yyyy').format(task.date)),
        _DetailRow(icon: Icons.schedule_outlined, label: 'Time', value: task.time),
        const SizedBox(height: AppSpacing.md),
        Row(
          children: [
            Expanded(
              child: AppButton(
                label: 'Call',
                icon: Icons.call_outlined,
                onPressed: () => showCallOutcomeFlow(
                  context,
                  name: task.customerName,
                  phone: task.customerPhone,
                  onProceed: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => VerificationFlowPage(
                        caseId: task.id,
                        customerName: task.customerName,
                        loanType: task.title,
                      ),
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AppButton(
                label: 'Share',
                icon: Icons.share_outlined,
                variant: AppButtonVariant.outline,
                onPressed: () => SharePlus.instance.share(
                  ShareParams(
                    text: '${task.title}\nCustomer: ${task.customerName}\nPhone: ${task.customerPhone}\n'
                        '${DateFormat('EEE, d MMM yyyy').format(task.date)} · ${task.time}',
                  ),
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _DetailRow extends StatelessWidget {
  const _DetailRow({required this.icon, required this.label, required this.value});

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Icon(icon, size: 18, color: AppColors.textSecondary),
          const SizedBox(width: AppSpacing.sm),
          SizedBox(width: 72, child: Text(label, style: Theme.of(context).textTheme.bodySmall)),
          Expanded(child: Text(value, style: Theme.of(context).textTheme.bodyMedium)),
        ],
      ),
    );
  }
}

class _QuickActionsGrid extends StatelessWidget {
  const _QuickActionsGrid();

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: quickActions.length,
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: AppSpacing.sm,
        crossAxisSpacing: AppSpacing.sm,
        childAspectRatio: 1.0,
      ),
      itemBuilder: (context, index) => QuickActionCard(action: quickActions[index]),
    );
  }
}

/// The menu ("three lines") icon's drawer — previously a purely decorative
/// icon with no tap handler at all, so nothing happened when tapped.
class _AppDrawer extends StatelessWidget {
  const _AppDrawer({required this.name, required this.designation});

  final String name;
  final String designation;

  Future<void> _logout(BuildContext context) async {
    Navigator.of(context).pop();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Sign Out'),
        content: const Text('Are you sure you want to sign out of your ONEFIN employee account?'),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('Sign Out')),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    await context.read<AuthProvider>().logout();
    if (context.mounted) context.goNamed(RouteNames.login);
  }

  @override
  Widget build(BuildContext context) {
    return Drawer(
      child: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DrawerHeader(
              decoration: const BoxDecoration(color: AppColors.primary),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  CircleAvatar(
                    radius: 22,
                    backgroundColor: Colors.white,
                    child: Text(
                      name.isNotEmpty ? name.substring(0, 1).toUpperCase() : 'E',
                      style: const TextStyle(color: AppColors.primary, fontWeight: FontWeight.w700, fontSize: 18),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text(name, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w700, fontSize: 16)),
                  if (designation.isNotEmpty) Text(designation, style: const TextStyle(color: Colors.white70, fontSize: 12)),
                ],
              ),
            ),
            ListTile(
              leading: const Icon(Icons.dashboard_outlined),
              title: const Text('Dashboard'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.dashboard);
              },
            ),
            ListTile(
              leading: const Icon(Icons.folder_outlined),
              title: const Text('Cases'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.cases);
              },
            ),
            ListTile(
              leading: const Icon(Icons.payments_outlined),
              title: const Text('Collections'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.collections);
              },
            ),
            ListTile(
              leading: const Icon(Icons.notifications_outlined),
              title: const Text('Notifications'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.notifications);
              },
            ),
            ListTile(
              leading: const Icon(Icons.person_outline),
              title: const Text('Profile'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.profile);
              },
            ),
            const Spacer(),
            const Divider(height: 1),
            ListTile(
              leading: const Icon(Icons.logout, color: AppColors.danger),
              title: const Text('Sign Out', style: TextStyle(color: AppColors.danger)),
              onTap: () => _logout(context),
            ),
            const SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }
}
