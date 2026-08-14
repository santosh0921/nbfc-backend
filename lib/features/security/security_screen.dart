import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../auth/widgets/code_boxes.dart';

/// Profile → Security: change your MPIN (real 6-box flow, matching
/// pattern), toggle biometric login, and review active sessions.
/// Frontend-only — MPIN change is validated locally, nothing is sent
/// anywhere.
class SecurityScreen extends StatefulWidget {
  const SecurityScreen({super.key});

  @override
  State<SecurityScreen> createState() => _SecurityScreenState();
}

class _SecurityScreenState extends State<SecurityScreen> {
  bool _biometricEnabled = true;

  static const _sessions = [
    ('This Device', 'Android · Active now', true),
    ('iPhone 14', 'iOS · 2 days ago', false),
    ('Chrome — Windows', 'Web · 5 days ago', false),
  ];

  void _openChangeMpinSheet() {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (context) => const _ChangeMpinSheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Security')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  ListTile(
                    leading: const Icon(Icons.password_rounded, color: AppColors.primary),
                    title: const Text('Change MPIN'),
                    trailing: const Icon(Icons.chevron_right_rounded),
                    onTap: _openChangeMpinSheet,
                  ),
                  const Divider(height: 1, indent: 16),
                  SwitchListTile(
                    secondary: const Icon(Icons.fingerprint_rounded, color: AppColors.primary),
                    title: const Text('Biometric Login'),
                    subtitle: const Text('Use fingerprint or face unlock'),
                    value: _biometricEnabled,
                    activeThumbColor: AppColors.primary,
                    onChanged: (v) => setState(() => _biometricEnabled = v),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            Text('Active Sessions', style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (int i = 0; i < _sessions.length; i++) ...[
                    ListTile(
                      leading: Icon(
                        _sessions[i].$3 ? Icons.smartphone_rounded : Icons.devices_other_rounded,
                        color: _sessions[i].$3 ? AppColors.success : AppColors.textSecondaryLight,
                      ),
                      title: Text(_sessions[i].$1),
                      subtitle: Text(_sessions[i].$2),
                      trailing: _sessions[i].$3
                          ? Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                              decoration: BoxDecoration(
                                color: AppColors.success.withValues(alpha: 0.12),
                                borderRadius: BorderRadius.circular(AppRadius.pill),
                              ),
                              child: Text('This device', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.success)),
                            )
                          : TextButton(
                              onPressed: () => ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text('Signed out of ${_sessions[i].$1}.')),
                              ),
                              child: const Text('Sign out'),
                            ),
                    ),
                    if (i != _sessions.length - 1) const Divider(height: 1, indent: 16),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ChangeMpinSheet extends StatefulWidget {
  const _ChangeMpinSheet();

  @override
  State<_ChangeMpinSheet> createState() => _ChangeMpinSheetState();
}

class _ChangeMpinSheetState extends State<_ChangeMpinSheet> {
  String _current = '';
  String _newMpin = '';
  String _confirmMpin = '';
  String? _error;

  bool get _canSubmit => _current.length == 6 && _newMpin.length == 6 && _confirmMpin.length == 6;

  void _submit() {
    if (_confirmMpin != _newMpin) {
      setState(() => _error = 'New MPIN and confirmation do not match.');
      return;
    }
    Navigator.of(context).pop();
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('MPIN updated successfully.')));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(24, 20, 24, 28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Change MPIN', style: theme.textTheme.headlineSmall),
            const SizedBox(height: 20),
            Text('Current MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, autofocus: true, onChanged: (v) => setState(() => _current = v)),
            const SizedBox(height: 18),
            Text('New MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, onChanged: (v) => setState(() => _newMpin = v)),
            const SizedBox(height: 18),
            Text('Confirm New MPIN', style: theme.textTheme.labelMedium),
            const SizedBox(height: 8),
            CodeBoxes(obscure: true, hasError: _error != null, onChanged: (v) => setState(() {
                  _confirmMpin = v;
                  _error = null;
                })),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(_error!, style: theme.textTheme.bodySmall?.copyWith(color: AppColors.error)),
            ],
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: _canSubmit ? _submit : null,
              style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(52)),
              child: const Text('Update MPIN'),
            ),
          ],
        ),
      ),
    );
  }
}
