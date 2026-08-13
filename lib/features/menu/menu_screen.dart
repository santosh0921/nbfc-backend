import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/emi_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/mock_data.dart';

/// Full-screen menu opened from the hamburger icon on Home: greeting
/// card, a stack of navigation rows, and a row of 4 quick-icon shortcut
/// buttons — mirrors the structure of established NBFC-style app menus.
class MenuScreen extends StatelessWidget {
  const MenuScreen({super.key});

  /// The EMI Schedule route needs a specific loanId (see
  /// emi_schedule_screen.dart) — unlike every other Menu row, this entry
  /// point has no loan already in context, so it used to navigate with no
  /// loanId at all and hit "This loan could not be found." on literally
  /// every tap. Resolves a real loanId (the customer's current/most
  /// recent active loan) first, or tells the customer there's nothing to
  /// show instead of taking them to a guaranteed-broken screen.
  Future<void> _openEmiSchedule(BuildContext context) async {
    final token = AuthProvider.instance.token;
    if (token == null) return;
    try {
      final summary = await EmiApiService.dashboardSummary(token);
      if (!context.mounted) return;
      if (!summary.hasUpcomingEmi || summary.loanId == null) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('No active EMI schedule to show yet.')),
        );
        return;
      }
      context.push('/emi-schedule', extra: '${summary.loanId}');
    } on ApiException catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } catch (_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not load your EMI schedule right now.')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    const user = MockData.currentUser;

    return Scaffold(
      appBar: AppBar(
        actions: [
          IconButton(
            onPressed: () => context.push('/notifications'),
            icon: const Icon(Icons.notifications_none_rounded),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 4, 20, 32),
          children: [
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: AppColors.primary.withValues(alpha: 0.06),
                borderRadius: BorderRadius.circular(AppRadius.lg),
                border: Border.all(color: AppColors.primary.withValues(alpha: 0.16)),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      'Hello ${user.fullName.split(' ').first}',
                      style: theme.textTheme.titleLarge,
                    ),
                  ),
                  CircleAvatar(
                    radius: 24,
                    backgroundColor: AppColors.primary.withValues(alpha: 0.14),
                    child: Text(
                      user.avatarInitials,
                      style: const TextStyle(color: AppColors.primary, fontWeight: FontWeight.w800),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            _MenuRow(
              icon: Icons.person_rounded,
              label: 'Profile Details',
              onTap: () => context.go('/profile'),
            ),
            _MenuRow(
              icon: Icons.account_balance_wallet_rounded,
              label: 'My Loans',
              onTap: () => context.push('/my-loans'),
            ),
            _MenuRow(
              icon: Icons.badge_rounded,
              label: 'Track Application',
              onTap: () => context.push('/track-application'),
            ),
            _MenuRow(
              icon: Icons.event_repeat_rounded,
              label: 'EMI Payments',
              onTap: () => context.go('/emi-payments'),
            ),
            _MenuRow(
              icon: Icons.payments_rounded,
              label: 'Payments',
              onTap: () => context.push('/payments'),
            ),
            _MenuRow(
              icon: Icons.history_rounded,
              label: 'Transaction History',
              onTap: () => context.push('/transactions'),
            ),
            _MenuRow(
              icon: Icons.event_note_rounded,
              label: 'EMI Schedule',
              onTap: () => _openEmiSchedule(context),
            ),
            _MenuRow(
              icon: Icons.receipt_long_rounded,
              label: 'Utilities',
              onTap: () => context.push('/compliance'),
            ),
            _MenuRow(
              icon: Icons.admin_panel_settings_rounded,
              label: 'App Permissions',
              onTap: () => context.push('/permissions'),
            ),
            const SizedBox(height: 24),
            Row(
              children: [
                Expanded(
                  child: _QuickIconButton(
                    icon: Icons.calculate_rounded,
                    label: 'EMI\nCalculator',
                    onTap: () => context.push('/emi-calculator'),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _QuickIconButton(
                    icon: Icons.people_alt_rounded,
                    label: 'About\nUs',
                    onTap: () => context.push('/compliance'),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _QuickIconButton(
                    icon: Icons.location_on_rounded,
                    label: 'Locate\nBranch',
                    onTap: () => context.push('/support'),
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _QuickIconButton(
                    icon: Icons.help_outline_rounded,
                    label: 'FAQs &\nSupport',
                    onTap: () => context.push('/support'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _MenuRow extends StatelessWidget {
  const _MenuRow({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: PremiumCard(
        onTap: onTap,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: AppColors.secondary.withValues(alpha: 0.16),
                borderRadius: BorderRadius.circular(AppRadius.sm),
              ),
              child: Icon(icon, color: AppColors.secondaryDark, size: 20),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Text(label, style: Theme.of(context).textTheme.titleSmall),
            ),
            const Icon(Icons.chevron_right_rounded, color: AppColors.textSecondaryLight),
          ],
        ),
      ),
    );
  }
}

class _QuickIconButton extends StatelessWidget {
  const _QuickIconButton({required this.icon, required this.label, required this.onTap});

  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.primary,
      borderRadius: BorderRadius.circular(AppRadius.md),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.md),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 6),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, color: Colors.white, size: 22),
              const SizedBox(height: 6),
              Text(
                label,
                textAlign: TextAlign.center,
                style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w600, height: 1.2),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
