import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/widgets/shell_back_handler.dart';
import '../../../notifications/presentation/providers/notifications_provider.dart';

/// Bottom-navigation shell for the Recovery module — Home, Cases, Alerts,
/// Profile. Mirrors [DashboardShell]'s structure without the raised center
/// button since Recovery only needs 4 even tabs.
class RecoveryShell extends StatefulWidget {
  const RecoveryShell({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  static const _tabs = [
    (icon: Icons.space_dashboard_outlined, activeIcon: Icons.space_dashboard, label: 'Home'),
    (icon: Icons.folder_shared_outlined, activeIcon: Icons.folder_shared, label: 'Cases'),
    (icon: Icons.notifications_outlined, activeIcon: Icons.notifications, label: 'Alerts'),
    (icon: Icons.person_outline, activeIcon: Icons.person, label: 'Profile'),
  ];

  @override
  State<RecoveryShell> createState() => _RecoveryShellState();
}

class _RecoveryShellState extends State<RecoveryShell> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => context.read<NotificationsProvider>().load());
  }

  void _select(int index) {
    HapticFeedback.selectionClick();
    widget.navigationShell.goBranch(index, initialLocation: index == widget.navigationShell.currentIndex);
  }

  @override
  Widget build(BuildContext context) {
    final unreadCount = context.watch<NotificationsProvider>().unreadCount;
    final currentIndex = widget.navigationShell.currentIndex;
    final bottomInset = MediaQuery.of(context).padding.bottom;

    return ShellBackHandler(
      navigationShell: widget.navigationShell,
      child: Scaffold(
      body: widget.navigationShell,
      backgroundColor: Colors.white,
      bottomNavigationBar: SizedBox(
        height: 74 + bottomInset,
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: Colors.white,
            border: Border(top: BorderSide(color: AppColors.border)),
          ),
          child: Padding(
            padding: EdgeInsets.only(bottom: bottomInset),
            child: Row(
              children: [
                for (int i = 0; i < RecoveryShell._tabs.length; i++)
                  Expanded(
                    child: _NavItem(
                      icon: RecoveryShell._tabs[i].icon,
                      activeIcon: RecoveryShell._tabs[i].activeIcon,
                      label: RecoveryShell._tabs[i].label,
                      selected: currentIndex == i,
                      badgeCount: i == 2 ? unreadCount : 0,
                      onTap: () => _select(i),
                    ),
                  ),
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
  const _NavItem({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.selected,
    required this.onTap,
    this.badgeCount = 0,
  });

  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool selected;
  final int badgeCount;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Stack(
            clipBehavior: Clip.none,
            children: [
              AnimatedScale(
                scale: selected ? 1.1 : 1.0,
                duration: const Duration(milliseconds: 180),
                curve: Curves.easeOutBack,
                child: Icon(
                  selected ? activeIcon : icon,
                  color: selected ? AppColors.secondary : AppColors.textSecondary,
                  size: 24,
                ),
              ),
              if (badgeCount > 0)
                Positioned(
                  top: -4,
                  right: -8,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
                    decoration: const BoxDecoration(color: AppColors.danger, borderRadius: BorderRadius.all(Radius.circular(8))),
                    child: Text(
                      badgeCount > 9 ? '9+' : '$badgeCount',
                      style: const TextStyle(color: Colors.white, fontSize: 9, fontWeight: FontWeight.w700),
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          AnimatedDefaultTextStyle(
            duration: const Duration(milliseconds: 180),
            style: TextStyle(
              fontSize: 11,
              fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
              color: selected ? AppColors.secondary : AppColors.textSecondary,
            ),
            child: Text(label),
          ),
        ],
      ),
    );
  }
}
