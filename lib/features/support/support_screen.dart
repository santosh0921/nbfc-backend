import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/support_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../mock/mock_data.dart';
import '../../models/support_message.dart';

class _Complaint {
  final String id;
  final String category;
  final String description;
  final String status;
  final DateTime raisedOn;

  const _Complaint({
    required this.id,
    required this.category,
    required this.description,
    required this.status,
    required this.raisedOn,
  });
}

const _faqs = [
  ('How do I check my loan application status?', 'Go to Menu → Track Application, or check My Loans from the home screen for real-time status updates.'),
  ('How is my EMI calculated?', 'Your EMI is calculated using your loan amount, interest rate, and tenure — reducing balance method. Use the EMI Calculator on any loan\'s detail page to estimate it.'),
  ('Can I prepay or foreclose my loan?', 'Yes. Go to My Loans → select your loan → Part Payment or Foreclosure. Nil charges apply on floating-rate loans after 12 EMIs.'),
  ('How do I update my KYC documents?', 'Visit Profile → Documents, or re-upload your Aadhaar/PAN during your next application — updates are verified within 24 hours.'),
  ('What happens if I miss an EMI payment?', 'A grace period of 3 days applies before penal charges (2% p.m. on overdue amount) are levied. Repeated defaults are reported to credit bureaus.'),
];

/// Real Customer Support screen: searchable FAQs, a complaint form
/// wired to a local (mock) ticket list, direct contact details, and a
/// branch locator. No backend — complaints are stored in memory only
/// for this session.
class SupportScreen extends StatefulWidget {
  const SupportScreen({super.key});

  @override
  State<SupportScreen> createState() => _SupportScreenState();
}

class _SupportScreenState extends State<SupportScreen> with SingleTickerProviderStateMixin {
  late final TabController _tabController = TabController(length: 4, vsync: this);
  final List<_Complaint> _complaints = [
    _Complaint(
      id: 'TCK-88214',
      category: 'Payment Issue',
      description: 'EMI deducted twice for Personal Loan in the same billing cycle.',
      status: 'In Progress',
      raisedOn: DateTime.now().subtract(const Duration(days: 3)),
    ),
  ];

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  void _addComplaint(String category, String description) {
    setState(() {
      _complaints.insert(
        0,
        _Complaint(
          id: 'TCK-${90000 + _complaints.length}',
          category: category,
          description: description,
          status: 'Open',
          raisedOn: DateTime.now(),
        ),
      );
    });
    _tabController.animateTo(2);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Customer Support'),
        bottom: TabBar(
          controller: _tabController,
          labelColor: AppColors.primary,
          unselectedLabelColor: AppColors.textSecondaryLight,
          indicatorColor: AppColors.primary,
          isScrollable: true,
          tabs: const [Tab(text: 'FAQs'), Tab(text: 'Contact'), Tab(text: 'My Complaints'), Tab(text: 'Chat with Us')],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          const _FaqTab(),
          _ContactTab(onComplaintRaised: _addComplaint),
          _ComplaintsTab(complaints: _complaints),
          const _SupportChatTab(),
        ],
      ),
    );
  }
}

class _FaqTab extends StatelessWidget {
  const _FaqTab();

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      padding: const EdgeInsets.all(20),
      itemCount: _faqs.length,
      separatorBuilder: (_, __) => const SizedBox(height: 10),
      itemBuilder: (context, i) {
        final (q, a) = _faqs[i];
        return Theme(
          data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
          child: PremiumCard(
            padding: EdgeInsets.zero,
            child: ExpansionTile(
              title: Text(q, style: Theme.of(context).textTheme.titleSmall),
              childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
              children: [
                Align(alignment: Alignment.centerLeft, child: Text(a, style: Theme.of(context).textTheme.bodyMedium)),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _ContactTab extends StatefulWidget {
  const _ContactTab({required this.onComplaintRaised});

  final void Function(String category, String description) onComplaintRaised;

  @override
  State<_ContactTab> createState() => _ContactTabState();
}

class _ContactTabState extends State<_ContactTab> {
  final _formKey = GlobalKey<FormState>();
  final _descriptionController = TextEditingController();
  String _category = 'Payment Issue';

  static const _categories = ['Payment Issue', 'Application Status', 'KYC / Documents', 'Technical Issue', 'Other'];

  @override
  void dispose() {
    _descriptionController.dispose();
    super.dispose();
  }

  void _copy(String value, String label) {
    Clipboard.setData(ClipboardData(text: value));
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label copied to clipboard')));
  }

  Future<void> _callUs() async {
    final uri = Uri.parse('tel:+918657823643');
    final launched = await launchUrl(uri);
    if (!launched && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not open the dialer. Please call 8657823643 directly.')),
      );
    }
  }

  void _submit() {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    widget.onComplaintRaised(_category, _descriptionController.text.trim());
    _descriptionController.clear();
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Complaint raised successfully.')));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ListView(
      padding: const EdgeInsets.all(20),
      children: [
        Text('Reach Us Directly', style: theme.textTheme.titleMedium),
        const SizedBox(height: 12),
        Material(
          color: AppColors.primary,
          borderRadius: BorderRadius.circular(AppRadius.md),
          child: InkWell(
            borderRadius: BorderRadius.circular(AppRadius.md),
            onTap: _callUs,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 18),
              child: Row(
                children: [
                  Container(
                    width: 46,
                    height: 46,
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.16),
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(Icons.phone_in_talk_rounded, color: Colors.white),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Call Us',
                          style: theme.textTheme.titleMedium?.copyWith(color: Colors.white, fontWeight: FontWeight.w700),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '8657823643 · Tap to call now',
                          style: theme.textTheme.bodySmall?.copyWith(color: Colors.white.withValues(alpha: 0.9)),
                        ),
                      ],
                    ),
                  ),
                  const Icon(Icons.chevron_right_rounded, color: Colors.white),
                ],
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),
        PremiumCard(
          padding: EdgeInsets.zero,
          child: Column(
            children: [
              ListTile(
                leading: const Icon(Icons.call_rounded, color: AppColors.primary),
                title: const Text('1800-266-4545'),
                subtitle: const Text('Toll-free · Mon–Sat, 9 AM–7 PM'),
                trailing: const Icon(Icons.copy_rounded, size: 18),
                onTap: () => _copy('1800-266-4545', 'Phone number'),
              ),
              const Divider(height: 1, indent: 16),
              ListTile(
                leading: const Icon(Icons.email_rounded, color: AppColors.primary),
                title: const Text('support@nbfcpremium.in'),
                trailing: const Icon(Icons.copy_rounded, size: 18),
                onTap: () => _copy('support@nbfcpremium.in', 'Email address'),
              ),
              const Divider(height: 1, indent: 16),
              ListTile(
                leading: const Icon(Icons.chat_rounded, color: AppColors.success),
                title: const Text('+91 98765 43200'),
                subtitle: const Text('WhatsApp support'),
                trailing: const Icon(Icons.copy_rounded, size: 18),
                onTap: () => _copy('+91 98765 43200', 'WhatsApp number'),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        Text('Raise a Complaint', style: theme.textTheme.titleMedium),
        const SizedBox(height: 12),
        Form(
          key: _formKey,
          child: PremiumCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Category', style: theme.textTheme.titleSmall),
                const SizedBox(height: 8),
                DropdownButtonFormField<String>(
                  value: _category,
                  items: [for (final c in _categories) DropdownMenuItem(value: c, child: Text(c))],
                  onChanged: (v) => setState(() => _category = v ?? _category),
                  decoration: InputDecoration(
                    filled: true,
                    fillColor: AppColors.backgroundLight,
                    contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
                    border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
                    enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
                    focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.primary, width: 1.6)),
                  ),
                ),
                const SizedBox(height: 16),
                Text('Description', style: theme.textTheme.titleSmall),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _descriptionController,
                  maxLines: 4,
                  validator: (v) => (v == null || v.trim().isEmpty) ? 'Please describe your issue' : null,
                  decoration: InputDecoration(
                    hintText: 'Describe your issue in detail…',
                    filled: true,
                    fillColor: AppColors.backgroundLight,
                    contentPadding: const EdgeInsets.all(14),
                    border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
                    enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.borderLight)),
                    focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(AppRadius.sm), borderSide: const BorderSide(color: AppColors.primary, width: 1.6)),
                  ),
                ),
                const SizedBox(height: 16),
                ElevatedButton(
                  onPressed: _submit,
                  style: ElevatedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
                  child: const Text('Submit Complaint'),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 24),
        Text('Branch Locator', style: theme.textTheme.titleMedium),
        const SizedBox(height: 12),
        PremiumCard(
          padding: EdgeInsets.zero,
          child: Column(
            children: [
              for (int i = 0; i < MockData.branches.length; i++) ...[
                ListTile(
                  leading: const Icon(Icons.location_on_rounded, color: AppColors.primary),
                  title: Text(MockData.branches[i]),
                  subtitle: const Text('10 AM – 6 PM · Mon–Sat'),
                ),
                if (i != MockData.branches.length - 1) const Divider(height: 1, indent: 16),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

class _ComplaintsTab extends StatelessWidget {
  const _ComplaintsTab({required this.complaints});

  final List<_Complaint> complaints;

  Color _statusColor(String status) => switch (status) {
        'Resolved' => AppColors.success,
        'In Progress' => AppColors.warning,
        _ => AppColors.primary,
      };

  @override
  Widget build(BuildContext context) {
    if (complaints.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.inbox_rounded, size: 48, color: AppColors.textSecondaryLight),
              const SizedBox(height: 12),
              Text('No complaints raised yet.', style: Theme.of(context).textTheme.bodyMedium),
            ],
          ),
        ),
      );
    }
    final theme = Theme.of(context);
    return ListView.separated(
      padding: const EdgeInsets.all(20),
      itemCount: complaints.length,
      separatorBuilder: (_, __) => const SizedBox(height: 10),
      itemBuilder: (context, i) {
        final c = complaints[i];
        return PremiumCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(c.id, style: theme.textTheme.titleSmall),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: _statusColor(c.status).withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(AppRadius.pill),
                    ),
                    child: Text(c.status, style: theme.textTheme.labelSmall?.copyWith(color: _statusColor(c.status), fontWeight: FontWeight.w700)),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(c.category, style: theme.textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 4),
              Text(c.description, style: theme.textTheme.bodySmall),
              const SizedBox(height: 8),
              Text('Raised ${_relativeDate(c.raisedOn)}', style: theme.textTheme.labelSmall),
            ],
          ),
        );
      },
    );
  }

  String _relativeDate(DateTime date) {
    final days = DateTime.now().difference(date).inDays;
    if (days == 0) return 'today';
    if (days == 1) return 'yesterday';
    return '$days days ago';
  }
}

/// Real support chat wired to `GET`/`POST /auth/support/messages`
/// (`internal/support/handler.go`) — the customer's own thread with the
/// full message history, message bubbles (customer vs admin), a text
/// input to send new messages, and pull-to-refresh for near-real-time
/// updates without polling infrastructure.
class _SupportChatTab extends StatefulWidget {
  const _SupportChatTab();

  @override
  State<_SupportChatTab> createState() => _SupportChatTabState();
}

class _SupportChatTabState extends State<_SupportChatTab> {
  late Future<SupportThreadResponse> _future;
  final _messageController = TextEditingController();
  final _scrollController = ScrollController();
  bool _sending = false;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  Future<SupportThreadResponse> _load() async {
    final token = AuthProvider.instance.token;
    if (token == null) return const SupportThreadResponse(thread: null, messages: []);
    return SupportApiService.list(token);
  }

  Future<void> _refresh() async {
    setState(() => _future = _load());
    await _future;
  }

  void _scrollToBottom() {
    if (!_scrollController.hasClients) return;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 250),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _send() async {
    final text = _messageController.text.trim();
    if (text.isEmpty || _sending) return;
    final token = AuthProvider.instance.token;
    if (token == null) return;
    setState(() => _sending = true);
    try {
      await SupportApiService.send(token, text);
      _messageController.clear();
      await _refresh();
      _scrollToBottom();
    } on ApiException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(
          child: FutureBuilder<SupportThreadResponse>(
            future: _future,
            builder: (context, snapshot) {
              if (snapshot.connectionState == ConnectionState.waiting) {
                return const Center(child: CircularProgressIndicator());
              }
              if (snapshot.hasError) {
                final message =
                    snapshot.error is ApiException ? (snapshot.error as ApiException).message : 'Could not load your messages.';
                return RefreshIndicator(
                  onRefresh: _refresh,
                  child: ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    children: [
                      SizedBox(
                        height: 300,
                        child: Center(
                          child: Padding(
                            padding: const EdgeInsets.all(24),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.error_outline_rounded, size: 48, color: AppColors.error),
                                const SizedBox(height: 12),
                                Text(message, style: Theme.of(context).textTheme.bodyMedium, textAlign: TextAlign.center),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                );
              }
              final data = snapshot.data ?? const SupportThreadResponse(thread: null, messages: []);
              final messages = data.messages;
              if (messages.isEmpty) {
                return RefreshIndicator(
                  onRefresh: _refresh,
                  child: ListView(
                    physics: const AlwaysScrollableScrollPhysics(),
                    children: [
                      SizedBox(
                        height: 300,
                        child: Center(
                          child: Padding(
                            padding: const EdgeInsets.all(24),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                const Icon(Icons.chat_bubble_outline_rounded, size: 48, color: AppColors.textSecondaryLight),
                                const SizedBox(height: 12),
                                Text(
                                  'No messages yet — send one to get help',
                                  style: Theme.of(context).textTheme.bodyMedium,
                                  textAlign: TextAlign.center,
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                );
              }
              _scrollToBottom();
              return RefreshIndicator(
                onRefresh: _refresh,
                child: ListView.builder(
                  controller: _scrollController,
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
                  itemCount: messages.length,
                  itemBuilder: (context, i) => _ChatBubble(message: messages[i]),
                ),
              );
            },
          ),
        ),
        SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: TextField(
                    controller: _messageController,
                    minLines: 1,
                    maxLines: 4,
                    textCapitalization: TextCapitalization.sentences,
                    decoration: InputDecoration(
                      hintText: 'Type a message…',
                      filled: true,
                      fillColor: AppColors.backgroundLight,
                      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.pill),
                        borderSide: const BorderSide(color: AppColors.borderLight),
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.pill),
                        borderSide: const BorderSide(color: AppColors.borderLight),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.pill),
                        borderSide: const BorderSide(color: AppColors.primary, width: 1.6),
                      ),
                    ),
                    onSubmitted: (_) => _send(),
                  ),
                ),
                const SizedBox(width: 8),
                Material(
                  color: AppColors.primary,
                  shape: const CircleBorder(),
                  child: InkWell(
                    customBorder: const CircleBorder(),
                    onTap: _sending ? null : _send,
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: _sending
                          ? const SizedBox(
                              height: 20,
                              width: 20,
                              child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                            )
                          : const Icon(Icons.send_rounded, color: Colors.white, size: 20),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _ChatBubble extends StatelessWidget {
  const _ChatBubble({required this.message});

  final SupportMessage message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isCustomer = message.isCustomer;
    final bubbleColor = isCustomer ? AppColors.primary : AppColors.backgroundLight;
    final textColor = isCustomer ? Colors.white : AppColors.textPrimaryLight;

    return Align(
      alignment: isCustomer ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.78),
        child: Column(
          crossAxisAlignment: isCustomer ? CrossAxisAlignment.end : CrossAxisAlignment.start,
          children: [
            if (!isCustomer)
              Padding(
                padding: const EdgeInsets.only(left: 4, bottom: 2),
                child: Text('Support Team', style: theme.textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight)),
              ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              decoration: BoxDecoration(
                color: bubbleColor,
                borderRadius: BorderRadius.only(
                  topLeft: const Radius.circular(16),
                  topRight: const Radius.circular(16),
                  bottomLeft: Radius.circular(isCustomer ? 16 : 4),
                  bottomRight: Radius.circular(isCustomer ? 4 : 16),
                ),
              ),
              child: Text(
                message.body,
                style: theme.textTheme.bodyMedium?.copyWith(color: textColor),
              ),
            ),
            Padding(
              padding: const EdgeInsets.only(top: 3, left: 4, right: 4),
              child: Text(
                DateFormat('d MMM, h:mm a').format(message.createdAt),
                style: theme.textTheme.labelSmall?.copyWith(color: AppColors.textSecondaryLight),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
