import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';

/// Formats raw digits into Aadhaar's standard "XXXX XXXX XXXX" grouping
/// as the customer types, so the field reads the way an Aadhaar number
/// actually looks instead of one unbroken 12-digit string.
class _AadhaarGroupFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(TextEditingValue oldValue, TextEditingValue newValue) {
    final digits = newValue.text.replaceAll(RegExp(r'\D'), '');
    final limited = digits.length > 12 ? digits.substring(0, 12) : digits;

    final buffer = StringBuffer();
    for (int i = 0; i < limited.length; i++) {
      if (i != 0 && i % 4 == 0) buffer.write(' ');
      buffer.write(limited[i]);
    }

    return TextEditingValue(
      text: buffer.toString(),
      selection: TextSelection.collapsed(offset: buffer.length),
    );
  }
}

/// Aadhaar number entry styled to actually look like an Aadhaar card as
/// the customer types — a visual anchor for anyone who isn't confident
/// reading the plain-text label "Aadhaar Number", since the number
/// filling into a recognizable card shape confirms "yes, this is my
/// Aadhaar" the way a bare text field can't.
class AadhaarNumberField extends StatelessWidget {
  const AadhaarNumberField({super.key, required this.controller, required this.validator});

  final TextEditingController controller;
  final String? Function(String?) validator;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _AadhaarCardPreview(controller: controller),
        const SizedBox(height: 12),
        TextFormField(
          controller: controller,
          keyboardType: TextInputType.number,
          maxLength: 14, // 12 digits + 2 grouping spaces
          inputFormatters: [FilteringTextInputFormatter.digitsOnly, _AadhaarGroupFormatter()],
          style: const TextStyle(letterSpacing: 2, fontWeight: FontWeight.w600),
          decoration: const InputDecoration(
            labelText: 'Aadhaar Number',
            hintText: '1234 5678 9012',
            counterText: '',
            prefixIcon: Icon(Icons.badge_outlined),
          ),
          validator: (v) => validator(v?.replaceAll(' ', '')),
        ),
      ],
    );
  }
}

class _AadhaarCardPreview extends StatelessWidget {
  const _AadhaarCardPreview({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<TextEditingValue>(
      valueListenable: controller,
      builder: (context, value, _) {
        final digits = value.text.replaceAll(' ', '');
        final groups = <String>[
          digits.isNotEmpty ? digits.substring(0, digits.length.clamp(0, 4)) : '',
          digits.length > 4 ? digits.substring(4, digits.length.clamp(4, 8)) : '',
          digits.length > 8 ? digits.substring(8, digits.length.clamp(8, 12)) : '',
        ];

        return AspectRatio(
          aspectRatio: 1.586, // standard ID-card ratio
          child: Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppRadius.lg),
              gradient: const LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [Color(0xFFFF9933), Color(0xFFFFFFFF), Color(0xFF138808)],
                stops: [0.0, 0.5, 1.0],
              ),
              boxShadow: [
                BoxShadow(color: Colors.black.withValues(alpha: 0.15), blurRadius: 12, offset: const Offset(0, 6)),
              ],
            ),
            child: Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: Colors.white.withValues(alpha: 0.92),
                borderRadius: BorderRadius.circular(AppRadius.md),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 22,
                        height: 22,
                        decoration: const BoxDecoration(shape: BoxShape.circle, color: Color(0xFF1E3A8A)),
                        child: const Icon(Icons.account_balance_rounded, color: Colors.white, size: 13),
                      ),
                      const SizedBox(width: 8),
                      const Expanded(
                        child: Text(
                          'भारत सरकार · GOVERNMENT OF INDIA',
                          style: TextStyle(fontSize: 9.5, fontWeight: FontWeight.w700, color: Color(0xFF1E3A8A)),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                  const Spacer(),
                  Row(
                    children: [
                      Container(
                        width: 46,
                        height: 46,
                        decoration: BoxDecoration(
                          color: AppColors.borderLight,
                          borderRadius: BorderRadius.circular(6),
                          border: Border.all(color: AppColors.borderLight),
                        ),
                        child: const Icon(Icons.person_rounded, color: AppColors.textSecondaryLight, size: 30),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            for (final g in groups)
                              Padding(
                                padding: const EdgeInsets.only(bottom: 1),
                                child: SizedBox(
                                  height: 15,
                                  child: Row(
                                    children: [
                                      for (int i = 0; i < 4; i++)
                                        Container(
                                          margin: const EdgeInsets.only(right: 4),
                                          width: 13,
                                          alignment: Alignment.center,
                                          child: Text(
                                            i < g.length ? g[i] : '•',
                                            style: TextStyle(
                                              fontSize: 15,
                                              fontWeight: FontWeight.w800,
                                              letterSpacing: 1,
                                              color: i < g.length ? const Color(0xFF1E3A8A) : AppColors.borderLight,
                                            ),
                                          ),
                                        ),
                                    ],
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                    ],
                  ),
                  const Spacer(),
                  const Text(
                    'AADHAAR',
                    style: TextStyle(fontSize: 9, fontWeight: FontWeight.w800, letterSpacing: 2.5, color: Color(0xFFFF9933)),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
