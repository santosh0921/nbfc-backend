import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/theme/app_shadows.dart';

/// Premium 6-box OTP / MPIN entry. Supports paste, auto-advance,
/// auto-backspace across boxes, a focus scale animation, and an
/// [obscure] mode that shows animated filled dots instead of digits.
class CodeBoxes extends StatefulWidget {
  const CodeBoxes({
    super.key,
    this.length = 6,
    this.obscure = false,
    this.autofocus = false,
    this.hasError = false,
    this.onChanged,
    this.onCompleted,
  });

  final int length;
  final bool obscure;
  final bool autofocus;
  final bool hasError;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onCompleted;

  @override
  State<CodeBoxes> createState() => _CodeBoxesState();
}

class _CodeBoxesState extends State<CodeBoxes> with SingleTickerProviderStateMixin {
  late final List<TextEditingController> _controllers =
      List.generate(widget.length, (_) => TextEditingController());
  late final List<FocusNode> _focusNodes = List.generate(widget.length, (_) => FocusNode());

  late final AnimationController _shakeController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 380),
  );
  late final Animation<double> _shake = TweenSequence<double>([
    TweenSequenceItem(tween: Tween(begin: 0.0, end: -8.0), weight: 1),
    TweenSequenceItem(tween: Tween(begin: -8.0, end: 8.0), weight: 1),
    TweenSequenceItem(tween: Tween(begin: 8.0, end: -6.0), weight: 1),
    TweenSequenceItem(tween: Tween(begin: -6.0, end: 0.0), weight: 1),
  ]).animate(CurvedAnimation(parent: _shakeController, curve: Curves.easeOut));

  @override
  void didUpdateWidget(covariant CodeBoxes oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.hasError && !oldWidget.hasError) {
      _shakeController.forward(from: 0);
    }
  }

  @override
  void dispose() {
    for (final c in _controllers) {
      c.dispose();
    }
    for (final f in _focusNodes) {
      f.dispose();
    }
    _shakeController.dispose();
    super.dispose();
  }

  void _emit() {
    final code = _controllers.map((c) => c.text).join();
    widget.onChanged?.call(code);
    if (code.length == widget.length) {
      widget.onCompleted?.call(code);
      FocusScope.of(context).unfocus();
    }
  }

  void _handleChange(int index, String raw) {
    final digits = raw.replaceAll(RegExp(r'[^0-9]'), '');

    if (digits.length > 1) {
      var cursor = index;
      for (final digit in digits.split('')) {
        if (cursor >= widget.length) break;
        _controllers[cursor].text = digit;
        cursor++;
      }
      final lastFilled = (index + digits.length).clamp(0, widget.length) - 1;
      if (lastFilled >= 0 && lastFilled < widget.length - 1) {
        _focusNodes[lastFilled + 1].requestFocus();
      } else {
        FocusScope.of(context).unfocus();
      }
      _emit();
      return;
    }

    _controllers[index].text = digits;
    _controllers[index].selection = TextSelection.collapsed(offset: digits.length);

    if (digits.isNotEmpty && index < widget.length - 1) {
      _focusNodes[index + 1].requestFocus();
    }
    _emit();
  }

  void _handleBackspace(int index) {
    if (_controllers[index].text.isNotEmpty) {
      _controllers[index].clear();
      _emit();
      return;
    }
    if (index > 0) {
      _focusNodes[index - 1].requestFocus();
      _controllers[index - 1].clear();
      _emit();
    }
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _shake,
      builder: (context, child) => Transform.translate(offset: Offset(_shake.value, 0), child: child),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: List.generate(widget.length, (index) {
          return _Box(
            controller: _controllers[index],
            focusNode: _focusNodes[index],
            obscure: widget.obscure,
            hasError: widget.hasError,
            autofocus: widget.autofocus && index == 0,
            onChanged: (value) => _handleChange(index, value),
            onBackspace: () => _handleBackspace(index),
          );
        }),
      ),
    );
  }
}

class _Box extends StatefulWidget {
  const _Box({
    required this.controller,
    required this.focusNode,
    required this.obscure,
    required this.hasError,
    required this.autofocus,
    required this.onChanged,
    required this.onBackspace,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final bool obscure;
  final bool hasError;
  final bool autofocus;
  final ValueChanged<String> onChanged;
  final VoidCallback onBackspace;

  @override
  State<_Box> createState() => _BoxState();
}

class _BoxState extends State<_Box> {
  @override
  void initState() {
    super.initState();
    widget.focusNode.addListener(_onFocusChange);
    // Attach key handling directly to the FocusNode rather than wrapping
    // it in a separate KeyboardListener — KeyboardListener builds its own
    // internal Focus widget around whatever FocusNode it's given, and
    // TextField/EditableText does the same. Two Focus widgets attaching
    // the *same* FocusNode at two different tree positions corrupts the
    // focus parent chain into a cycle, which makes FocusNode.ancestors
    // loop forever and crash with an OutOfMemoryError the moment this
    // field gains focus. Setting onKeyEvent on the node itself avoids
    // ever wrapping it in a second Focus.
    widget.focusNode.onKeyEvent = _handleKeyEvent;
  }

  void _onFocusChange() => setState(() {});

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.backspace && widget.controller.text.isEmpty) {
      widget.onBackspace();
      return KeyEventResult.handled;
    }
    return KeyEventResult.ignored;
  }

  @override
  void dispose() {
    widget.focusNode.removeListener(_onFocusChange);
    widget.focusNode.onKeyEvent = null;
    super.dispose();
  }

  Color get _borderColor {
    if (widget.hasError) return AppColors.error;
    if (widget.focusNode.hasFocus) return AppColors.primary;
    return AppColors.borderLight;
  }

  @override
  Widget build(BuildContext context) {
    final filled = widget.controller.text.isNotEmpty;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return AnimatedScale(
      scale: widget.focusNode.hasFocus ? 1.04 : 1.0,
      duration: const Duration(milliseconds: 140),
      curve: Curves.easeOut,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 160),
        width: 48,
        height: 56,
        decoration: BoxDecoration(
          color: isDark ? AppColors.surfaceDark : AppColors.surfaceLight,
          borderRadius: BorderRadius.circular(AppRadius.md),
          border: Border.all(
            color: _borderColor,
            width: widget.focusNode.hasFocus || widget.hasError ? 1.8 : 1.2,
          ),
          boxShadow: widget.focusNode.hasFocus ? AppShadows.primaryGlow() : AppShadows.subtle,
        ),
        alignment: Alignment.center,
        child: Stack(
          alignment: Alignment.center,
          children: [
            if (widget.obscure)
              AnimatedScale(
                scale: filled ? 1 : 0,
                duration: const Duration(milliseconds: 180),
                curve: Curves.easeOutBack,
                child: const _Dot(),
              ),
            Opacity(
              opacity: widget.obscure ? 0 : 1,
              child: TextField(
                controller: widget.controller,
                focusNode: widget.focusNode,
                autofocus: widget.autofocus,
                textAlign: TextAlign.center,
                keyboardType: TextInputType.number,
                maxLength: widget.obscure ? 1 : 6,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: AppColors.textPrimaryLight),
                decoration: const InputDecoration(
                  counterText: '',
                  contentPadding: EdgeInsets.zero,
                  border: InputBorder.none,
                  enabledBorder: InputBorder.none,
                  focusedBorder: InputBorder.none,
                ),
                onChanged: widget.onChanged,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Dot extends StatelessWidget {
  const _Dot();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 11,
      height: 11,
      decoration: const BoxDecoration(color: AppColors.textPrimaryLight, shape: BoxShape.circle),
    );
  }
}
