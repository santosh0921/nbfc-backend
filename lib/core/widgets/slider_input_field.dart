import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

/// A labeled value + slider, with a typed number field next to the value
/// so users can either drag or type a value directly — typing "36"
/// immediately moves the slider to 36, and dragging updates the field.
class SliderInputField extends StatefulWidget {
  const SliderInputField({
    super.key,
    required this.label,
    required this.value,
    required this.min,
    required this.max,
    required this.onChanged,
    this.divisions,
    this.suffix,
    this.displayFormatter,
    this.fieldWidth = 108,
  });

  final String label;
  final double value;
  final double min;
  final double max;
  final ValueChanged<double> onChanged;
  final int? divisions;

  /// Shown after the typed number, e.g. " months".
  final String? suffix;

  /// If set, overrides the plain number shown in the field's read-out
  /// (the field itself always edits a plain number) — used for the
  /// slider's own inline value label bubble.
  final String Function(double)? displayFormatter;
  final double fieldWidth;

  @override
  State<SliderInputField> createState() => _SliderInputFieldState();
}

class _SliderInputFieldState extends State<SliderInputField> {
  late final TextEditingController _controller = TextEditingController(text: widget.value.round().toString());
  late final FocusNode _focusNode = FocusNode();

  @override
  void initState() {
    super.initState();
    _focusNode.addListener(() {
      // Re-sync the field to the clamped slider value once the user taps away.
      if (!_focusNode.hasFocus) {
        _controller.text = widget.value.round().toString();
      }
    });
  }

  @override
  void didUpdateWidget(covariant SliderInputField oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Keep the field in sync when the slider itself changes the value,
    // but don't fight the user while they're actively typing.
    if (!_focusNode.hasFocus && oldWidget.value != widget.value) {
      final text = widget.value.round().toString();
      if (_controller.text != text) _controller.text = text;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onTyped(String text) {
    final parsed = double.tryParse(text);
    if (parsed == null) return;
    final clamped = parsed.clamp(widget.min, widget.max);
    widget.onChanged(clamped);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(widget.label, style: theme.textTheme.bodyMedium),
            SizedBox(
              width: widget.fieldWidth,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Flexible(
                    child: TextField(
                      controller: _controller,
                      focusNode: _focusNode,
                      textAlign: TextAlign.right,
                      keyboardType: TextInputType.number,
                      inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                      style: theme.textTheme.titleSmall,
                      decoration: InputDecoration(
                        isDense: true,
                        contentPadding: const EdgeInsets.symmetric(vertical: 6, horizontal: 8),
                        filled: true,
                        fillColor: AppColors.primary.withValues(alpha: 0.06),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(AppRadius.xs),
                          borderSide: BorderSide.none,
                        ),
                      ),
                      onChanged: _onTyped,
                      onSubmitted: _onTyped,
                    ),
                  ),
                  if (widget.suffix != null) ...[
                    const SizedBox(width: 4),
                    Text(widget.suffix!, style: theme.textTheme.bodySmall),
                  ],
                ],
              ),
            ),
          ],
        ),
        Slider(
          value: widget.value.clamp(widget.min, widget.max),
          min: widget.min,
          max: widget.max,
          divisions: widget.divisions,
          activeColor: AppColors.primary,
          label: widget.displayFormatter?.call(widget.value) ?? widget.value.round().toString(),
          onChanged: (v) {
            widget.onChanged(v);
            if (!_focusNode.hasFocus) _controller.text = v.round().toString();
          },
        ),
      ],
    );
  }
}
