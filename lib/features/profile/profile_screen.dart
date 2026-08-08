import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/providers/theme_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/section_header.dart';
import '../../mock/mock_data.dart';
import '../home/widgets/transaction_tile.dart';

class ProfileScreen extends StatelessWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final user = MockData.currentUser;
    final theme = Theme.of(context);
    final themeProvider = context.watch<ThemeProvider>();

    return Scaffold(
      appBar: AppBar(title: const Text('Profile')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 120),
          children: [
            PremiumCard(
              child: Row(
                children: [
                  CircleAvatar(
                    radius: 30,
                    backgroundColor: AppColors.primary.withValues(alpha: 0.12),
                    child: Text(user.avatarInitials, style: const TextStyle(color: AppColors.primary, fontWeight: FontWeight.w800, fontSize: 20)),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(user.fullName, style: theme.textTheme.titleMedium),
                        Text(user.email, style: theme.textTheme.bodySmall),
                        const SizedBox(height: 6),
                        Row(
                          children: [
                            Icon(
                              user.kycVerified ? Icons.verified_rounded : Icons.error_outline_rounded,
                              size: 14,
                              color: user.kycVerified ? AppColors.success : AppColors.warning,
                            ),
                            const SizedBox(width: 4),
                            Text(
                              user.kycVerified ? 'KYC Verified' : 'KYC Pending',
                              style: theme.textTheme.labelSmall?.copyWith(
                                color: user.kycVerified ? AppColors.success : AppColors.warning,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            SectionHeader(
              title: 'Recent Transactions',
              actionLabel: 'View all',
              onAction: () => context.push('/transactions'),
            ),
            const SizedBox(height: 4),
            PremiumCard(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Column(
                children: [
                  for (final t in MockData.recentTransactions.take(3)) TransactionTile(transaction: t),
                ],
              ),
            ),
            const SizedBox(height: 20),
            _SectionCard(
              title: 'Account',
              items: [
                _ProfileTile(icon: Icons.description_rounded, label: 'Documents', onTap: () => context.push('/documents')),
                _ProfileTile(icon: Icons.security_rounded, label: 'Security', onTap: () => context.push('/security')),
                _ProfileTile(icon: Icons.notifications_none_rounded, label: 'Notifications', onTap: () => context.push('/notifications')),
                _ProfileTile(icon: Icons.language_rounded, label: 'Language', onTap: () => context.push('/language')),
              ],
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Preferences',
              items: [
                _ProfileTile(
                  icon: Icons.dark_mode_rounded,
                  label: 'Dark Mode',
                  trailing: Switch(
                    value: themeProvider.isDark,
                    activeColor: AppColors.primary,
                    onChanged: (_) => themeProvider.toggle(),
                  ),
                  onTap: themeProvider.toggle,
                ),
              ],
            ),
            const SizedBox(height: 16),
            _SectionCard(
              title: 'Legal & Support',
              items: [
                _ProfileTile(icon: Icons.support_agent_rounded, label: 'Customer Support', onTap: () => context.push('/support')),
                _ProfileTile(icon: Icons.verified_user_rounded, label: 'RBI Compliance', onTap: () => context.push('/compliance')),
                _ProfileTile(
                  icon: Icons.logout_rounded,
                  label: 'Logout',
                  color: AppColors.error,
                  onTap: () {
                    AuthProvider.instance.logout();
                    context.go('/login');
                  },
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  final String title;
  final List<_ProfileTile> items;

  const _SectionCard({required this.title, required this.items});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 4, bottom: 8),
          child: Text(title, style: Theme.of(context).textTheme.labelMedium),
        ),
        PremiumCard(
          padding: EdgeInsets.zero,
          child: Column(
            children: [
              for (int i = 0; i < items.length; i++) ...[
                items[i],
                if (i != items.length - 1) const Divider(height: 1, indent: 56),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

class _ProfileTile extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;
  final Widget? trailing;
  final Color? color;

  const _ProfileTile({required this.icon, required this.label, required this.onTap, this.trailing, this.color});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: onTap,
      leading: Icon(icon, color: color ?? AppColors.primary),
      title: Text(label, style: Theme.of(context).textTheme.titleSmall?.copyWith(color: color)),
      trailing: trailing ?? const Icon(Icons.chevron_right_rounded),
    );
  }
}
