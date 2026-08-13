import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import '../../mock/mock_data.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';
import '../widgets/dashboard_tour.dart';
import '../widgets/incoming_call_listener.dart';

/// Bottom-nav app shell hosting the 4 primary tabs (Home, Loans,
/// EMI Payments, Profile) plus a floating Quick Apply FAB, per the
/// StatefulShellRoute branch navigator passed in by [navigationShell].
///
/// Back-button behavior: on every bottom-nav tab (Home, Loans, EMI,
/// Profile), back requires a second press within 2s to actually exit the
/// app — first press just shows a confirmation snackbar.
class AppShell extends StatefulWidget {
  final StatefulNavigationShell navigationShell;

  const AppShell({super.key, required this.navigationShell});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  static const _tabs = [
    (icon: Icons.home_rounded, outlinedIcon: Icons.home_outlined, label: 'Home'),
    (icon: Icons.account_balance_rounded, outlinedIcon: Icons.account_balance_outlined, label: 'Loans'),
    (icon: Icons.event_repeat_rounded, outlinedIcon: Icons.event_repeat_outlined, label: 'EMI'),
    (icon: Icons.person_rounded, outlinedIcon: Icons.person_outline_rounded, label: 'Profile'),
  ];

  DateTime? _lastBackPress;

  Future<void> _handleBack() async {
    final now = DateTime.now();
    if (_lastBackPress != null && now.difference(_lastBackPress!) < const Duration(seconds: 2)) {
      SystemNavigator.pop();
      return;
    }
    _lastBackPress = now;
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        const SnackBar(
          content: Text('Press back again to exit'),
          duration: Duration(seconds: 2),
          behavior: SnackBarBehavior.floating,
        ),
      );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final navigationShell = widget.navigationShell;

    return IncomingCallListener(
      child: PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, result) {
          if (didPop) return;
          _handleBack();
        },
        child: Scaffold(
          extendBody: true,
          body: navigationShell,
          floatingActionButtonLocation: FloatingActionButtonLocation.centerDocked,
          floatingActionButton: FloatingActionButton(
            key: DashboardTourKeys.fab,
            onPressed: () {
              final matches = MockData.loanProducts.where((p) => p.id == 'instant_loan');
              context.push('/quick-apply', extra: matches.isEmpty ? null : matches.first);
            },
            shape: const CircleBorder(),
            child: const Icon(Icons.bolt_rounded),
          ),
          bottomNavigationBar: BottomAppBar(
            shape: const CircularNotchedRectangle(),
            notchMargin: 8,
            color: isDark ? AppColors.surfaceDark : AppColors.surfaceLight,
            elevation: 0,
            padding: EdgeInsets.zero,
            child: SizedBox(
              height: 64,
              child: Row(
                children: [
                  for (int i = 0; i < _tabs.length; i++) ...[
                    if (i == 2) const SizedBox(width: 40),
                    Expanded(
                      child: _NavItem(
                        key: i == 3 ? DashboardTourKeys.navProfile : null,
                        icon: navigationShell.currentIndex == i ? _tabs[i].icon : _tabs[i].outlinedIcon,
                        label: _tabs[i].label,
                        selected: navigationShell.currentIndex == i,
                        onTap: () => navigationShell.goBranch(
                          i,
                          initialLocation: i == navigationShell.currentIndex,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _NavItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  const _NavItem({super.key, required this.icon, required this.label, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final color = selected ? AppColors.primary : Theme.of(context).textTheme.bodySmall?.color;
    return InkWell(
      borderRadius: BorderRadius.circular(AppRadius.sm),
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, color: color, size: 24),
            const SizedBox(height: 2),
            Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: Theme.of(context).textTheme.labelSmall?.copyWith(
                    color: color,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                  ),
            ),
          ],
        ),
      ),
    );
  }
}
