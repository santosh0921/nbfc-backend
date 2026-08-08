import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import '../../../core/widgets/premium_card.dart';
import 'apply_form_field.dart';

/// Plain data for one loan reference, matching the backend's
/// `ReferenceInput` shape exactly (`internal/loans/reference.go`):
/// `{name, relation, phone, aadhaarNumber, panNumber}`.
class ReferenceEntry {
  const ReferenceEntry({
    required this.name,
    required this.relation,
    required this.phone,
    required this.aadhaarNumber,
    required this.panNumber,
  });

  final String name;
  final String relation;
  final String phone;
  final String aadhaarNumber;
  final String panNumber;

  Map<String, dynamic> toJson() => {
        'name': name,
        'relation': relation,
        'phone': phone,
        'aadhaarNumber': aadhaarNumber,
        'panNumber': panNumber,
      };
}

class _RefUpperCaseFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(TextEditingValue oldValue, TextEditingValue newValue) {
    return newValue.copyWith(text: newValue.text.toUpperCase(), selection: newValue.selection);
  }
}

const _relationOptions = ['Friend', 'Family', 'Colleague', 'Neighbour', 'Other'];

/// Collects EXACTLY 2 loan references — required by the backend on both
/// `POST /auth/loans/apply` and `POST /auth/loans/:id/topup`
/// (`internal/loans/reference.go` `validateReferences`: exactly 2, each
/// with name/relation/phone/12-digit Aadhaar/valid-format PAN). Shared
/// between the Quick Apply wizard and the "Request Top-Up" bottom sheet
/// so both stay in sync with the same contract.
class ReferencesFormWidget extends StatefulWidget {
  const ReferencesFormWidget({super.key});

  @override
  State<ReferencesFormWidget> createState() => ReferencesFormWidgetState();
}

class ReferencesFormWidgetState extends State<ReferencesFormWidget> {
  final _formKey = GlobalKey<FormState>();
  final _ref1 = _SingleReferenceData();
  final _ref2 = _SingleReferenceData();

  /// Validates both reference forms; returns true only if both pass.
  ///
  /// [Form.validate] marks every invalid field with an error, but with two
  /// stacked 5-field cards in a scroll view the first error is often below
  /// the fold — the user sees no error on screen and, having genuinely
  /// filled the fields, has no idea what's still wrong. So on failure we
  /// also focus the first invalid field, which makes the scroll view (and
  /// the keyboard) jump straight to it.
  bool validate() {
    final formValid = _formKey.currentState?.validate() ?? false;
    if (!formValid) {
      (_ref1.firstInvalidFocusNode() ?? _ref2.firstInvalidFocusNode())?.requestFocus();
    }
    return formValid;
  }

  /// Only meaningful once [validate] has returned true.
  List<ReferenceEntry> get references => [_ref1.toEntry(), _ref2.toEntry()];

  @override
  void dispose() {
    _ref1.dispose();
    _ref2.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Form(
      key: _formKey,
      autovalidateMode: AutovalidateMode.onUserInteraction,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Loan References', style: theme.textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(
            'Provide details of 2 people who can vouch for you. This is required for every loan application.',
            style: theme.textTheme.bodySmall,
          ),
          const SizedBox(height: 16),
          _SingleReferenceCard(index: 1, data: _ref1),
          const SizedBox(height: 16),
          _SingleReferenceCard(index: 2, data: _ref2),
        ],
      ),
    );
  }
}

class _SingleReferenceData {
  final nameController = TextEditingController();
  final phoneController = TextEditingController();
  final aadhaarController = TextEditingController();
  final panController = TextEditingController();
  final customRelationController = TextEditingController();
  String relation = 'Friend';

  final nameFocus = FocusNode();
  final customRelationFocus = FocusNode();
  final phoneFocus = FocusNode();
  final aadhaarFocus = FocusNode();
  final panFocus = FocusNode();

  ReferenceEntry toEntry() => ReferenceEntry(
        name: nameController.text.trim(),
        relation: relation == 'Other' ? customRelationController.text.trim() : relation,
        phone: phoneController.text.trim(),
        aadhaarNumber: aadhaarController.text.trim(),
        panNumber: panController.text.trim().toUpperCase(),
      );

  /// Re-runs each field's own validator, in on-screen order, and returns
  /// the [FocusNode] of the first one that's still invalid — or null if
  /// this reference is fully valid. Used to jump the user straight to
  /// whatever's actually wrong instead of leaving them to hunt for a
  /// possibly off-screen error.
  FocusNode? firstInvalidFocusNode() {
    if (nameController.text.trim().isEmpty) return nameFocus;
    if (relation == 'Other' && customRelationController.text.trim().isEmpty) return customRelationFocus;
    if (phoneController.text.trim().length != 10) return phoneFocus;
    if (aadhaarController.text.trim().length != 12) return aadhaarFocus;
    if (!RegExp(r'^[A-Z]{5}[0-9]{4}[A-Z]$').hasMatch(panController.text.trim().toUpperCase())) return panFocus;
    return null;
  }

  void dispose() {
    nameController.dispose();
    phoneController.dispose();
    aadhaarController.dispose();
    panController.dispose();
    customRelationController.dispose();
    nameFocus.dispose();
    customRelationFocus.dispose();
    phoneFocus.dispose();
    aadhaarFocus.dispose();
    panFocus.dispose();
  }
}

class _SingleReferenceCard extends StatefulWidget {
  const _SingleReferenceCard({required this.index, required this.data});

  final int index;
  final _SingleReferenceData data;

  @override
  State<_SingleReferenceCard> createState() => _SingleReferenceCardState();
}

class _SingleReferenceCardState extends State<_SingleReferenceCard> {
  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final data = widget.data;
    return PremiumCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Reference ${widget.index}', style: theme.textTheme.titleSmall),
          const SizedBox(height: 12),
          ApplyFormField(
            label: 'Full Name',
            controller: data.nameController,
            focusNode: data.nameFocus,
            validator: (v) => (v == null || v.trim().isEmpty) ? "Enter the reference's full name" : null,
          ),
          ApplyFormDropdown<String>(
            label: 'Relation',
            value: data.relation,
            items: _relationOptions,
            labelBuilder: (v) => v,
            onChanged: (v) => setState(() => data.relation = v ?? data.relation),
          ),
          if (data.relation == 'Other')
            ApplyFormField(
              label: 'Specify Relation',
              controller: data.customRelationController,
              focusNode: data.customRelationFocus,
              validator: (v) => (v == null || v.trim().isEmpty) ? 'Please specify the relation' : null,
            ),
          ApplyFormField(
            label: 'Phone Number',
            controller: data.phoneController,
            focusNode: data.phoneFocus,
            keyboardType: TextInputType.number,
            maxLength: 10,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) => (v == null || v.trim().length != 10) ? 'Enter a valid 10-digit phone number' : null,
          ),
          ApplyFormField(
            label: 'Aadhaar Number',
            controller: data.aadhaarController,
            focusNode: data.aadhaarFocus,
            hint: 'Must be exactly 12 digits',
            keyboardType: TextInputType.number,
            maxLength: 12,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            validator: (v) => (v == null || v.trim().length != 12) ? 'Enter a valid 12-digit Aadhaar number' : null,
          ),
          ApplyFormField(
            label: 'PAN Number',
            controller: data.panController,
            focusNode: data.panFocus,
            hint: 'Format: ABCDE1234F',
            maxLength: 10,
            inputFormatters: [_RefUpperCaseFormatter()],
            validator: (v) {
              if (v == null || v.trim().isEmpty) return 'Enter the PAN number';
              if (!RegExp(r'^[A-Z]{5}[0-9]{4}[A-Z]$').hasMatch(v.trim())) return 'Enter a valid PAN (e.g. ABCDE1234F)';
              return null;
            },
          ),
        ],
      ),
    );
  }
}
