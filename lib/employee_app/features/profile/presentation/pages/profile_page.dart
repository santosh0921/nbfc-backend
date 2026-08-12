import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/components/app_card.dart';
import '../../../../core/components/app_scaffold.dart';
import '../../../../core/components/gradient_header.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/constants/app_spacing.dart';
import '../../../../core/router/route_names.dart';
import '../../../../core/widgets/app_button.dart';
import '../../../auth/presentation/providers/auth_provider.dart';

class ProfilePage extends StatelessWidget {
  const ProfilePage({super.key});

  Future<void> _logout(BuildContext context) async {
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
    final employee = context.watch<AuthProvider>().employee;

    return AppScaffold(
      showAppBar: false,
      body: CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: GradientHeader(
              title: employee?.name ?? 'Employee',
              subtitle: '${employee?.designation ?? ''} · ${employee?.employeeCode ?? ''}',
              trailing: CircleAvatar(
                radius: 26,
                backgroundColor: AppColors.secondary,
                child: Text(
                  (employee?.name ?? 'E').substring(0, 1).toUpperCase(),
                  style: const TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.w700),
                ),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.all(AppSpacing.md),
            sliver: SliverList(
              delegate: SliverChildListDelegate([
                const Row(
                  children: [
                    Expanded(child: _StatCard(label: 'Attendance', value: '96%', icon: Icons.event_available_outlined)),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(child: _StatCard(label: 'Performance', value: '4.6/5', icon: Icons.trending_up_outlined)),
                  ],
                ),
                const SizedBox(height: AppSpacing.md),
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('This Month', style: Theme.of(context).textTheme.titleMedium),
                      const SizedBox(height: AppSpacing.sm),
                      const _MetricRow(label: 'Days Present', value: '24 / 25'),
                      const _MetricRow(label: 'Cases Closed', value: '38'),
                      const _MetricRow(label: 'Collections Made', value: '₹4,82,300'),
                      const _MetricRow(label: 'Avg. Turnaround', value: '1.4 days'),
                    ],
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                const AppCard(
                  padding: EdgeInsets.zero,
                  child: Column(
                    children: [
                      ListTile(
                        leading: Icon(Icons.notifications_outlined, color: AppColors.primary),
                        title: Text('Notification Preferences'),
                        trailing: Icon(Icons.chevron_right),
                      ),
                      Divider(height: 1),
                      ListTile(
                        leading: Icon(Icons.lock_outline, color: AppColors.primary),
                        title: Text('Change Password'),
                        trailing: Icon(Icons.chevron_right),
                      ),
                      Divider(height: 1),
                      ListTile(
                        leading: Icon(Icons.help_outline, color: AppColors.primary),
                        title: Text('Help & Support'),
                        trailing: Icon(Icons.chevron_right),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                AppButton(
                  label: 'Sign Out',
                  variant: AppButtonVariant.danger,
                  icon: Icons.logout,
                  onPressed: () => _logout(context),
                ),
                const SizedBox(height: AppSpacing.xxl),
              ]),
            ),
          ),
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  const _StatCard({required this.label, required this.value, required this.icon});

  final String label;
  final String value;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: AppColors.gold, size: 22),
          const SizedBox(height: 8),
          FittedBox(
            fit: BoxFit.scaleDown,
            alignment: Alignment.centerLeft,
            child: Text(value, style: Theme.of(context).textTheme.headlineSmall, maxLines: 1),
          ),
          Text(label, style: Theme.of(context).textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}

class _MetricRow extends StatelessWidget {
  const _MetricRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Flexible(
            child: Text(label, style: Theme.of(context).textTheme.bodyMedium, maxLines: 1, overflow: TextOverflow.ellipsis),
          ),
          const SizedBox(width: AppSpacing.sm),
          Text(value, style: Theme.of(context).textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}
