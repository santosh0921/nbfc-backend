import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

/// Shared back-button handling for the employee app's role shells
/// ([DashboardShell] for Verification, [RecoveryShell] for Recovery).
///
/// Both shells are [StatefulShellRoute.indexedStack]-based bottom-nav
/// shells where each branch owns its own nested [Navigator]. That nested
/// Navigator already intercepts the system back button and pops its own
/// stack first (e.g. Case Detail -> Cases List, or step 3 -> step 2 of a
/// visit wizard) — go_router wires up a [ChildBackButtonDispatcher] per
/// branch for exactly this, so in-flow back navigation keeps working
/// unmodified by this widget.
///
/// The back press only reaches this widget once the *active* branch's own
/// stack has nothing left to pop (i.e. the user is sitting at a tab root
/// such as Cases List, Alerts, or Profile). At that point:
///   * If the active tab is not [homeIndex], switch to the home tab.
///   * If the active tab already is [homeIndex] (the true top level of the
///     module), require a second back press within 2 seconds to exit,
///     showing a "Press back again to exit" SnackBar on the first press.
class ShellBackHandler extends StatefulWidget {
  const ShellBackHandler({
    super.key,
    required this.navigationShell,
    required this.child,
    this.homeIndex = 0,
  });

  final StatefulNavigationShell navigationShell;
  final Widget child;
  final int homeIndex;

  @override
  State<ShellBackHandler> createState() => _ShellBackHandlerState();
}

class _ShellBackHandlerState extends State<ShellBackHandler> {
  DateTime? _lastBackPress;

  void _handleBack() {
    final isHome = widget.navigationShell.currentIndex == widget.homeIndex;
    if (!isHome) {
      widget.navigationShell.goBranch(widget.homeIndex);
      return;
    }

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
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        _handleBack();
      },
      child: widget.child,
    );
  }
}
