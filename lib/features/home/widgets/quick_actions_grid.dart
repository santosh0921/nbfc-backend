import 'dart:async';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/widgets/press_scale.dart';

class QuickAction {
  final String label;
  final IconData icon;
  final VoidCallback onTap;

  const QuickAction({required this.label, required this.icon, required this.onTap});
}

/// Auto-scrolling, looping row of quick-action tiles that the user can
/// also freely drag through at any time — dragging pauses the auto-scroll
/// while a finger is down, and it resumes a moment after release. The
/// list is duplicated so the loop point is never visible, and the scroll
/// resets silently (no animation) once it passes one full copy.
class QuickActionsGrid extends StatefulWidget {
  final List<QuickAction> actions;

  const QuickActionsGrid({super.key, required this.actions});

  @override
  State<QuickActionsGrid> createState() => _QuickActionsGridState();
}

class _QuickActionsGridState extends State<QuickActionsGrid> {
  final ScrollController _scrollController = ScrollController();
  Timer? _ticker;
  Timer? _resumeTimer;
  bool _userInteracting = false;
  static const double _tileWidth = 92;
  double _loopExtent = 0;

  @override
  void initState() {
    super.initState();
    _loopExtent = _tileWidth * widget.actions.length;
    if (widget.actions.length > 1) {
      _startTicker();
    }
  }

  void _startTicker() {
    _ticker?.cancel();
    _ticker = Timer.periodic(const Duration(milliseconds: 30), (_) {
      if (!_scrollController.hasClients || _userInteracting) return;
      final next = _scrollController.offset + 0.6;
      if (next >= _loopExtent) {
        _scrollController.jumpTo(next - _loopExtent);
      } else {
        _scrollController.jumpTo(next);
      }
    });
  }

  void _onUserScrollStart() {
    _userInteracting = true;
    _resumeTimer?.cancel();
  }

  void _onUserScrollEnd() {
    _resumeTimer?.cancel();
    _resumeTimer = Timer(const Duration(seconds: 2), () {
      if (!mounted) return;
      // Normalize the offset back into the middle copy's range so the
      // auto-scroll doesn't visibly snap after a long manual drag.
      if (_scrollController.hasClients) {
        final offset = _scrollController.offset % _loopExtent;
        _scrollController.jumpTo(offset);
      }
      _userInteracting = false;
    });
  }

  @override
  void dispose() {
    _ticker?.cancel();
    _resumeTimer?.cancel();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.actions.isEmpty) return const SizedBox.shrink();
    // Three copies back-to-back so the visible window never runs out of
    // content while scrolling, and the jump-back is invisible.
    final looped = [...widget.actions, ...widget.actions, ...widget.actions];

    return SizedBox(
      height: 96,
      child: NotificationListener<ScrollNotification>(
        onNotification: (notification) {
          if (notification is ScrollStartNotification && notification.dragDetails != null) {
            _onUserScrollStart();
          } else if (notification is ScrollEndNotification) {
            _onUserScrollEnd();
          }
          return false;
        },
        child: ListView.builder(
          controller: _scrollController,
          scrollDirection: Axis.horizontal,
          physics: const BouncingScrollPhysics(),
          itemCount: looped.length,
          itemBuilder: (context, i) => SizedBox(
            width: _tileWidth,
            child: _QuickActionTile(action: looped[i]),
          ),
        ),
      ),
    );
  }
}

class _QuickActionTile extends StatelessWidget {
  const _QuickActionTile({required this.action});

  final QuickAction action;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return PressScale(
      onTap: action.onTap,
      child: Column(
        children: [
          Container(
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              color: isDark ? AppColors.surfaceDark : const Color(0xFFEFF3FA),
              borderRadius: BorderRadius.circular(AppRadius.md),
            ),
            child: Icon(action.icon, color: AppColors.primary, size: 24),
          ),
          const SizedBox(height: 6),
          Text(
            action.label,
            textAlign: TextAlign.center,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: Theme.of(context).textTheme.labelSmall,
          ),
        ],
      ),
    );
  }
}
