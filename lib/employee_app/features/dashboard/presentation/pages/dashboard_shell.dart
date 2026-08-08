import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../../../../core/constants/app_colors.dart';
import '../../../../core/widgets/shell_back_handler.dart';
import '../../../notifications/presentation/providers/notifications_provider.dart';

/// Bottom-navigation shell hosting the 5 primary employee tabs — Home,
/// Cases, [Collections — raised center], Alerts, Profile — each backed by
/// its own feature module. The raised black center button mirrors the
/// L&T Finance app's floating nav action.
class DashboardShell extends StatefulWidget {
  const DashboardShell({super.key, required this.navigationShell});

  final StatefulNavigationShell navigationShell;

  static const _tabs = [
    (icon: Icons.space_dashboard_outlined, activeIcon: Icons.space_dashboard, label: 'Home'),
    (icon: Icons.folder_shared_outlined, activeIcon: Icons.folder_shared, label: 'Cases'),
    (icon: Icons.payments_outlined, activeIcon: Icons.payments, label: 'Collections'),
    (icon: Icons.notifications_outlined, activeIcon: Icons.notifications, label: 'Alerts'),
    (icon: Icons.person_outline, activeIcon: Icons.person, label: 'Profile'),
  ];

  static const _centerIndex = 2;

  @override
  State<DashboardShell> createState() => _DashboardShellState();
}

class _DashboardShellState extends State<DashboardShell> {
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
    // Adapts to whichever system navigation the device uses: ~0 on 3-button
    // nav (its own reserved system bar), >0 on gesture nav (overlay inset).
    final bottomInset = MediaQuery.of(context).padding.bottom;

    return ShellBackHandler(
      navigationShell: widget.navigationShell,
      child: Scaffold(
      body: widget.navigationShell,
      backgroundColor: Colors.white,
      bottomNavigationBar: SizedBox(
        height: 74 + bottomInset,
        child: Stack(
          clipBehavior: Clip.none,
          alignment: Alignment.topCenter,
          children: [
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: Colors.white,
                  border: Border(top: BorderSide(color: AppColors.border)),
                ),
                child: Padding(
                  padding: EdgeInsets.only(bottom: bottomInset),
                  child: Row(
                    children: [
                      for (int i = 0; i < DashboardShell._tabs.length; i++)
                        Expanded(
                          child: i == DashboardShell._centerIndex
                              ? const SizedBox.shrink()
                              : _NavItem(
                                  icon: DashboardShell._tabs[i].icon,
                                  activeIcon: DashboardShell._tabs[i].activeIcon,
                                  label: DashboardShell._tabs[i].label,
                                  selected: currentIndex == i,
                                  badgeCount: i == 3 ? unreadCount : 0,
                                  onTap: () => _select(i),
                                ),
                        ),
                    ],
                  ),
                ),
              ),
            ),
            Positioned(
              top: -22,
              child: _CenterNavButton(
                icon: DashboardShell._tabs[DashboardShell._centerIndex].icon,
                selected: currentIndex == DashboardShell._centerIndex,
                onTap: () => _select(DashboardShell._centerIndex),
              ),
            ),
          ],
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

class _CenterNavButton extends StatelessWidget {
  const _CenterNavButton({required this.icon, required this.selected, required this.onTap});

  final IconData icon;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 220),
        curve: Curves.easeOutCubic,
        width: 58,
        height: 58,
        decoration: BoxDecoration(
          color: AppColors.secondary,
          shape: BoxShape.circle,
          border: Border.all(color: Colors.white, width: 4),
          boxShadow: [BoxShadow(color: AppColors.secondary.withValues(alpha: 0.3), blurRadius: 16, offset: const Offset(0, 6))],
        ),
        child: Icon(icon, color: selected ? AppColors.primary : Colors.white, size: 26),
      ),
    );
  }
}
