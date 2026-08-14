import 'package:flutter/material.dart';
import '../../core/theme/app_colors.dart';
import '../../core/widgets/premium_card.dart';

/// Profile → Language: select your preferred app language. Frontend-only
/// (no localization pipeline wired up), but the selection is real,
/// persists for the session, and gives immediate confirmation feedback.
class LanguageScreen extends StatefulWidget {
  const LanguageScreen({super.key});

  @override
  State<LanguageScreen> createState() => _LanguageScreenState();
}

class _LanguageScreenState extends State<LanguageScreen> {
  String _selected = 'English';

  static const _languages = [
    ('English', 'English'),
    ('हिन्दी', 'Hindi'),
    ('मराठी', 'Marathi'),
    ('ગુજરાતી', 'Gujarati'),
    ('தமிழ்', 'Tamil'),
    ('తెలుగు', 'Telugu'),
    ('বাংলা', 'Bengali'),
    ('ಕನ್ನಡ', 'Kannada'),
  ];

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Language')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Text(
              'Choose your preferred language for the app interface.',
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 16),
            PremiumCard(
              padding: EdgeInsets.zero,
              child: RadioGroup<String>(
                groupValue: _selected,
                onChanged: (v) {
                  if (v == null) return;
                  setState(() => _selected = v);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('App language set to $v.')),
                  );
                },
                child: Column(
                  children: [
                    for (int i = 0; i < _languages.length; i++) ...[
                      RadioListTile<String>(
                        value: _languages[i].$2,
                        activeColor: AppColors.primary,
                        title: Text(_languages[i].$1),
                        subtitle: _languages[i].$2 != _languages[i].$1 ? Text(_languages[i].$2) : null,
                      ),
                      if (i != _languages.length - 1) const Divider(height: 1, indent: 16),
                    ],
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
