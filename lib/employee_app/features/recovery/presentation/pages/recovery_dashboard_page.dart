import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../../core/widgets/premium_shine.dart';
import '../../../../../core/widgets/auto_refresh_mixin.dart';
import '../../../auth/presentation/providers/auth_provider.dart';
import '../../../notifications/presentation/providers/notifications_provider.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../providers/recovery_cases_provider.dart';
import '../providers/recovery_dashboard_provider.dart';

class RecoveryDashboardPage extends StatefulWidget {
  const RecoveryDashboardPage({super.key});

  @override
  State<RecoveryDashboardPage> createState() => _RecoveryDashboardPageState();
}

class _RecoveryDashboardPageState extends State<RecoveryDashboardPage> with AutoRefreshMixin {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<RecoveryDashboardProvider>().load());
    startAutoRefresh();
  }

  @override
  Future<void> onAutoRefresh() async {
    if (!mounted) return;
    await context.read<RecoveryDashboardProvider>().load(silent: true);
  }

  @override
  void dispose() {
    stopAutoRefresh();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final dashboard = context.watch<RecoveryDashboardProvider>();
    final unread = context.watch<NotificationsProvider>().unreadCount;
    final employee = auth.employee;
    final summary = dashboard.summary;

    return AppScaffold(
      showAppBar: false,
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
      drawer: _RecoveryDrawer(name: employee?.name ?? 'Recovery Officer', designation: employee?.designation ?? ''),
      body: dashboard.isLoading || summary == null
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: () => context.read<RecoveryDashboardProvider>().load(),
              child: ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                // See dashboard_home_page.dart — Clip.none stops the
                // default hardEdge clip from chopping card shadows off at
                // the list's own horizontal edge.
                clipBehavior: Clip.none,
                children: [
                  const SizedBox(height: AppSpacing.sm),
                  _TopBar(name: employee?.name ?? 'Recovery Officer', unreadCount: unread),
                  const SizedBox(height: AppSpacing.lg),
                  const _RecoveryHero(),
                  const SizedBox(height: AppSpacing.lg),
                  Text('Recovery Overview', style: Theme.of(context).textTheme.headlineSmall),
                  const SizedBox(height: AppSpacing.sm),
                  _KpiGrid(summary: summary),
                  const SizedBox(height: AppSpacing.lg),
                  Row(
                    children: [
                      Expanded(
                        child: _GaugeCard(
                          label: 'Recovery Performance',
                          percent: summary.recoveryPerformancePercent,
                          color: AppColors.success,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: _GaugeCard(
                          label: 'Collection Rate',
                          percent: summary.collectionPercent,
                          color: AppColors.info,
                        ),
                      ),
                    ],
                  ),
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
              Text(name, style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800), maxLines: 1, overflow: TextOverflow.ellipsis),
            ],
          ),
        ),
        InkWell(
          borderRadius: BorderRadius.circular(AppSpacing.radiusFull),
          onTap: () => context.goNamed(RouteNames.recoveryAlerts),
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

class _RecoveryHero extends StatelessWidget {
  const _RecoveryHero();

  @override
  Widget build(BuildContext context) {
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        gradient: const LinearGradient(colors: AppColors.heroGradient, begin: Alignment.topLeft, end: Alignment.bottomRight),
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
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
                      const Text(
                        'RECOVERY OFFICER',
                        style: TextStyle(color: AppColors.secondary, fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 0.4),
                      ),
                      const SizedBox(height: 6),
                      const Text(
                        'COLLECTIONS,\nMADE EASY.',
                        style: TextStyle(color: AppColors.secondary, fontSize: 20, fontWeight: FontWeight.w800, height: 1.15),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      InkWell(
                        onTap: () => context.goNamed(RouteNames.recoveryCases),
                        borderRadius: BorderRadius.circular(AppSpacing.radiusFull),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
                          decoration: BoxDecoration(color: AppColors.secondary, borderRadius: BorderRadius.circular(AppSpacing.radiusFull)),
                          child: const Text('View Cases', style: TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
                        ),
                      ),
                    ],
                  ),
                ),
                const Icon(Icons.savings_rounded, color: AppColors.secondary, size: 56),
              ],
            ),
          ),
          const Positioned.fill(child: PremiumShine(opacity: 0.3)),
        ],
      ),
    );
  }
}

class _KpiGrid extends StatelessWidget {
  const _KpiGrid({required this.summary});

  final RecoveryDashboardSummary summary;

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 2,
      mainAxisSpacing: AppSpacing.sm,
      crossAxisSpacing: AppSpacing.sm,
      childAspectRatio: 1.5,
      children: [
        _KpiCard(
          label: "Today's Assigned",
          value: summary.todaysAssignedCount,
          icon: Icons.assignment_ind_outlined,
          color: AppColors.info,
          onTap: () => context.goNamed(RouteNames.recoveryCases),
        ),
        _KpiCard(
          label: 'Pending Visits',
          value: summary.pendingVisitsCount,
          icon: Icons.event_available_outlined,
          color: AppColors.warning,
          onTap: () {
            context.read<RecoveryCasesProvider>().setPendingVisitsQuickFilter();
            context.goNamed(RouteNames.recoveryCases);
          },
        ),
        _KpiCard(
          label: 'Overdue Amount',
          value: summary.overdueAmount,
          icon: Icons.currency_rupee,
          color: AppColors.danger,
          isCurrency: true,
        ),
        _KpiCard(
          label: 'Promise To Pay',
          value: summary.promiseToPayCount,
          icon: Icons.handshake_outlined,
          color: AppColors.success,
          onTap: () {
            context.read<RecoveryCasesProvider>().setPromiseToPayQuickFilter();
            context.goNamed(RouteNames.recoveryCases);
          },
        ),
        _KpiCard(
          label: 'Broken Promises',
          value: summary.brokenPromiseCount,
          icon: Icons.report_gmailerrorred_outlined,
          color: AppColors.danger,
        ),
        _KpiCard(
          label: 'Closed Cases',
          value: summary.closedCasesCount,
          icon: Icons.task_alt_outlined,
          color: AppColors.success,
          onTap: () {
            context.read<RecoveryCasesProvider>().setClosedQuickFilter();
            context.goNamed(RouteNames.recoveryCases);
          },
        ),
      ],
    );
  }
}

class _KpiCard extends StatefulWidget {
  const _KpiCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
    this.onTap,
    this.isCurrency = false,
  });

  final String label;
  final num value;
  final IconData icon;
  final Color color;
  final VoidCallback? onTap;
  final bool isCurrency;

  @override
  State<_KpiCard> createState() => _KpiCardState();
}

class _KpiCardState extends State<_KpiCard> with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 900));
  late final Animation<double> _animation = CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic);

  @override
  void initState() {
    super.initState();
    _controller.forward();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  String _format(double v) {
    if (widget.isCurrency) {
      if (v >= 100000) return '₹${(v / 100000).toStringAsFixed(1)}L';
      if (v >= 1000) return '₹${(v / 1000).toStringAsFixed(1)}K';
      return '₹${v.toStringAsFixed(0)}';
    }
    return v.toStringAsFixed(0);
  }

  @override
  Widget build(BuildContext context) {
    return AppCard(
      onTap: widget.onTap,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 32,
                height: 32,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: widget.color.withValues(alpha: 0.12), borderRadius: BorderRadius.circular(AppSpacing.radiusSm)),
                child: Icon(widget.icon, color: widget.color, size: 18),
              ),
              const Spacer(),
            ],
          ),
          const Spacer(),
          AnimatedBuilder(
            animation: _animation,
            builder: (context, _) {
              final current = widget.value * _animation.value;
              return FittedBox(
                fit: BoxFit.scaleDown,
                alignment: Alignment.centerLeft,
                child: Text(
                  _format(current.toDouble()),
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
                  maxLines: 1,
                ),
              );
            },
          ),
          Text(widget.label, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}

class _GaugeCard extends StatelessWidget {
  const _GaugeCard({required this.label, required this.percent, required this.color});

  final String label;
  final double percent;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final clamped = (percent / 100).clamp(0.0, 1.0);
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: Theme.of(context).textTheme.bodySmall, maxLines: 2, overflow: TextOverflow.ellipsis),
          const SizedBox(height: AppSpacing.sm),
          Stack(
            alignment: Alignment.center,
            children: [
              SizedBox(
                width: 64,
                height: 64,
                child: TweenAnimationBuilder<double>(
                  tween: Tween(begin: 0, end: clamped),
                  duration: const Duration(milliseconds: 900),
                  curve: Curves.easeOutCubic,
                  builder: (context, value, _) => CircularProgressIndicator(
                    value: value,
                    strokeWidth: 6,
                    backgroundColor: AppColors.surfaceMuted,
                    valueColor: AlwaysStoppedAnimation(color),
                  ),
                ),
              ),
              Text('${percent.toStringAsFixed(0)}%', style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800)),
            ],
          ),
        ],
      ),
    );
  }
}

class _QuickActionsGrid extends StatelessWidget {
  const _QuickActionsGrid();

  static const _actions = [
    (icon: Icons.folder_shared_outlined, label: 'View Cases'),
    (icon: Icons.event_available_outlined, label: 'Pending Visits'),
    (icon: Icons.handshake_outlined, label: 'Promises'),
  ];

  @override
  Widget build(BuildContext context) {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: _actions.length,
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        mainAxisSpacing: AppSpacing.sm,
        crossAxisSpacing: AppSpacing.sm,
        childAspectRatio: 1.0,
      ),
      itemBuilder: (context, index) {
        final action = _actions[index];
        return AppCard(
          onTap: () {
            final provider = context.read<RecoveryCasesProvider>();
            switch (index) {
              case 1:
                provider.setPendingVisitsQuickFilter();
              case 2:
                provider.setPromiseToPayQuickFilter();
              default:
                provider.clearFilters();
            }
            context.goNamed(RouteNames.recoveryCases);
          },
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(action.icon, color: AppColors.secondary, size: 26),
              const SizedBox(height: AppSpacing.sm),
              Text(action.label, style: Theme.of(context).textTheme.bodySmall, textAlign: TextAlign.center, maxLines: 2, overflow: TextOverflow.ellipsis),
            ],
          ),
        );
      },
    );
  }
}

/// The menu icon's drawer for the Recovery role shell — previously a
/// purely decorative icon with no tap handler at all.
class _RecoveryDrawer extends StatelessWidget {
  const _RecoveryDrawer({required this.name, required this.designation});

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
                      name.isNotEmpty ? name.substring(0, 1).toUpperCase() : 'R',
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
                context.goNamed(RouteNames.recoveryDashboard);
              },
            ),
            ListTile(
              leading: const Icon(Icons.folder_outlined),
              title: const Text('Cases'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.recoveryCases);
              },
            ),
            ListTile(
              leading: const Icon(Icons.notifications_outlined),
              title: const Text('Alerts'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.recoveryAlerts);
              },
            ),
            ListTile(
              leading: const Icon(Icons.person_outline),
              title: const Text('Profile'),
              onTap: () {
                Navigator.of(context).pop();
                context.goNamed(RouteNames.recoveryProfile);
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
