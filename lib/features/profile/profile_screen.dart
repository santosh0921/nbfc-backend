import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/auth_api_service.dart';
import '../../core/network/transaction_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/providers/theme_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/premium_card.dart';
import '../../core/widgets/section_header.dart';
import '../../models/transaction.dart';
import '../home/widgets/transaction_tile.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  String _fullName = '';
  String _email = '';
  String _avatarInitials = '?';
  bool _kycCompleted = false;
  List<AppTransaction> _transactions = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// Best-effort, same non-blocking pattern used across the dashboard —
  /// a failure here (offline, profile not created yet) just leaves the
  /// fields at their empty defaults rather than blocking the screen.
  Future<void> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    try {
      final res = await AuthApiService.getProfile(token);
      final user = res['user'] as Map<String, dynamic>?;
      final first = (user?['first_name'] as String?)?.trim() ?? '';
      final last = (user?['last_name'] as String?)?.trim() ?? '';
      final fullName = [first, last].where((s) => s.isNotEmpty).join(' ');
      final initials = [first, last].where((s) => s.isNotEmpty).map((s) => s[0].toUpperCase()).join();
      if (!mounted) return;
      setState(() {
        _fullName = fullName;
        _email = (user?['email'] as String?)?.trim() ?? '';
        _avatarInitials = initials.isEmpty ? '?' : initials;
        _kycCompleted = user?['kyc_completed'] as bool? ?? false;
      });
    } on ApiException {
      // Non-fatal.
    } catch (_) {
      // Non-fatal.
    }
    try {
      final transactions = await TransactionApiService.mine(token);
      if (!mounted) return;
      setState(() => _transactions = transactions);
    } on ApiException {
      // Non-fatal.
    } catch (_) {
      // Non-fatal.
    }
  }

  @override
  Widget build(BuildContext context) {
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
                    child: Text(_avatarInitials, style: const TextStyle(color: AppColors.primary, fontWeight: FontWeight.w800, fontSize: 20)),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          _fullName.isEmpty ? 'Complete your profile' : _fullName,
                          style: theme.textTheme.titleMedium,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (_email.isNotEmpty) Text(_email, style: theme.textTheme.bodySmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                        const SizedBox(height: 6),
                        Row(
                          children: [
                            Icon(
                              _kycCompleted ? Icons.verified_rounded : Icons.error_outline_rounded,
                              size: 14,
                              color: _kycCompleted ? AppColors.success : AppColors.warning,
                            ),
                            const SizedBox(width: 4),
                            Text(
                              _kycCompleted ? 'KYC Verified' : 'KYC Pending',
                              style: theme.textTheme.labelSmall?.copyWith(
                                color: _kycCompleted ? AppColors.success : AppColors.warning,
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
            if (_transactions.isEmpty)
              PremiumCard(
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: Text(
                    'No transactions yet.',
                    style: theme.textTheme.bodySmall,
                    textAlign: TextAlign.center,
                  ),
                ),
              )
            else
              PremiumCard(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Column(
                  children: [
                    for (final t in _transactions.take(3)) TransactionTile(transaction: t),
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
                    activeThumbColor: AppColors.primary,
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
                  onTap: () => _confirmLogout(context),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _confirmLogout(BuildContext context) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Logout'),
        content: const Text('Are you sure you want to logout?'),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.of(context).pop(true), child: const Text('Logout')),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) return;
    AuthProvider.instance.logout();
    if (context.mounted) context.go('/login');
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
