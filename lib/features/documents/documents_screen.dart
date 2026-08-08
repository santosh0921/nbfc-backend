import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import 'package:printing/printing.dart';
import 'package:share_plus/share_plus.dart';
import '../../core/network/api_exception.dart';
import '../../core/network/document_api_service.dart';
import '../../core/providers/auth_provider.dart';
import '../../core/services/signed_documents_store.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_radius.dart';
import '../../core/widgets/premium_card.dart';
import '../../models/uploaded_document.dart';
import '../apply/widgets/document_upload_tile.dart';

/// Where a picked-file's source came from, for the upload bottom sheet.
enum _UploadSource { camera, files }

/// Sentinel returned by the doc-type picker when the user chooses "Other"
/// — the caller then prompts for free text instead of using this value.
const _kOtherDocType = '__other__';

/// Profile → Documents: view and re-upload your KYC documents any time,
/// not just during a loan application. Reuses the same real camera/
/// gallery/crop upload flow as the application wizard, and lets you
/// export any uploaded document — or all of them together — as a real
/// PDF via the system share/save sheet.
///
/// Also hosts "My Uploaded Documents" — real files uploaded to the
/// backend via `POST /auth/documents` (camera capture or existing
/// image/PDF picked from device storage), fetched back via
/// `GET /auth/documents` and viewable/downloadable/shareable inline.
class DocumentsScreen extends StatefulWidget {
  const DocumentsScreen({super.key});

  @override
  State<DocumentsScreen> createState() => _DocumentsScreenState();
}

class _DocumentsScreenState extends State<DocumentsScreen> {
  bool _aadhaar = true;
  bool _pan = true;
  bool _photo = false;

  File? _aadhaarFile;
  File? _panFile;
  File? _photoFile;

  List<SignedDocument> _signedDocs = [];

  // --- Uploaded documents (backend-backed) ---
  bool _loadingUploaded = true;
  String? _uploadedError;
  List<UploadedDocument> _uploadedDocs = [];
  bool _uploading = false;

  @override
  void initState() {
    super.initState();
    _loadSignedDocs();
    _loadUploadedDocs();
  }

  Future<void> _loadSignedDocs() async {
    final docs = await SignedDocumentsStore.list();
    if (!mounted) return;
    setState(() => _signedDocs = docs);
  }

  Future<void> _shareSignedDoc(SignedDocument doc) async {
    final file = File(doc.filePath);
    final exists = await file.exists();
    if (!mounted) return;
    if (!exists) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('This document is no longer available on this device.')),
      );
      return;
    }
    final bytes = await file.readAsBytes();
    await Printing.sharePdf(bytes: bytes, filename: '${doc.title.replaceAll(' ', '_')}.pdf');
  }

  Future<Uint8List> _buildPdf(List<(String label, File file)> docs) async {
    final pdf = pw.Document();
    for (final (label, file) in docs) {
      final bytes = await file.readAsBytes();
      final image = pw.MemoryImage(bytes);
      pdf.addPage(
        pw.Page(
          pageFormat: PdfPageFormat.a4,
          build: (context) => pw.Column(
            crossAxisAlignment: pw.CrossAxisAlignment.start,
            children: [
              pw.Text(label, style: pw.TextStyle(fontSize: 18, fontWeight: pw.FontWeight.bold)),
              pw.SizedBox(height: 16),
              pw.Expanded(child: pw.Center(child: pw.Image(image, fit: pw.BoxFit.contain))),
            ],
          ),
        ),
      );
    }
    return pdf.save();
  }

  Future<void> _saveAsPdf(String label, File? file) async {
    if (file == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Upload your $label first.')),
      );
      return;
    }
    final bytes = await _buildPdf([(label, file)]);
    await Printing.sharePdf(bytes: bytes, filename: '${label.replaceAll(' ', '_')}.pdf');
  }

  Future<void> _saveAllAsPdf() async {
    final docs = <(String, File)>[
      if (_aadhaarFile != null) ('Aadhaar Card', _aadhaarFile!),
      if (_panFile != null) ('PAN Card', _panFile!),
      if (_photoFile != null) ('Photograph', _photoFile!),
    ];
    if (docs.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Upload at least one document first.')),
      );
      return;
    }
    final bytes = await _buildPdf(docs);
    await Printing.sharePdf(bytes: bytes, filename: 'KYC_Documents.pdf');
  }

  // --- Uploaded documents (backend-backed) ---

  Future<void> _loadUploadedDocs() async {
    final token = AuthProvider.instance.token;
    if (token == null) {
      setState(() {
        _loadingUploaded = false;
        _uploadedError = 'Please log in again to view your documents.';
      });
      return;
    }
    setState(() {
      _loadingUploaded = true;
      _uploadedError = null;
    });
    try {
      final docs = await DocumentApiService.mine(token);
      if (!mounted) return;
      setState(() {
        _uploadedDocs = docs;
        _loadingUploaded = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _uploadedError = e.message;
        _loadingUploaded = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _uploadedError = 'Could not load your documents. Please try again.';
        _loadingUploaded = false;
      });
    }
  }

  String _mimeTypeFor(String fileName) {
    final ext = fileName.toLowerCase().split('.').last;
    switch (ext) {
      case 'png':
        return 'image/png';
      case 'jpg':
      case 'jpeg':
        return 'image/jpeg';
      case 'heic':
        return 'image/heic';
      case 'pdf':
        return 'application/pdf';
      default:
        return 'application/octet-stream';
    }
  }

  Future<String?> _pickDocType() {
    const presets = [
      'Aadhaar Card',
      'PAN Card',
      'Address Proof',
      'Income Proof',
      'Bank Statement',
      'Photo',
    ];
    return showModalBottomSheet<String>(
      context: context,
      backgroundColor: Colors.transparent,
      builder: (context) => Container(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 28),
        decoration: const BoxDecoration(
          color: AppColors.surfaceLight,
          borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(width: 40, height: 4, decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(4))),
            const SizedBox(height: 16),
            Text('What document is this?', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            for (final label in presets)
              ListTile(
                leading: const Icon(Icons.description_outlined, color: AppColors.primary),
                title: Text(label),
                onTap: () => Navigator.of(context).pop(label),
              ),
            ListTile(
              leading: const Icon(Icons.edit_outlined, color: AppColors.primary),
              title: const Text('Other'),
              onTap: () => Navigator.of(context).pop(_kOtherDocType),
            ),
          ],
        ),
      ),
    );
  }

  Future<String?> _promptCustomDocType() {
    final controller = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Document type'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: 'e.g. Rent Agreement'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
          TextButton(
            onPressed: () => Navigator.of(context).pop(controller.text.trim()),
            child: const Text('Continue'),
          ),
        ],
      ),
    );
  }

  Future<_UploadSource?> _pickSource() {
    return showModalBottomSheet<_UploadSource>(
      context: context,
      backgroundColor: Colors.transparent,
      builder: (context) => Container(
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 28),
        decoration: const BoxDecoration(
          color: AppColors.surfaceLight,
          borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(width: 40, height: 4, decoration: BoxDecoration(color: AppColors.borderLight, borderRadius: BorderRadius.circular(4))),
            const SizedBox(height: 16),
            Text('Add document', style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 16),
            ListTile(
              leading: const Icon(Icons.photo_camera_rounded, color: AppColors.primary),
              title: const Text('Take Photo'),
              onTap: () => Navigator.of(context).pop(_UploadSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.folder_open_rounded, color: AppColors.primary),
              title: const Text('Choose File (Image or PDF)'),
              onTap: () => Navigator.of(context).pop(_UploadSource.files),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _startUpload() async {
    var docType = await _pickDocType();
    if (docType == null) return;
    if (docType == _kOtherDocType) {
      docType = await _promptCustomDocType();
      if (docType == null || docType.trim().isEmpty) return;
    }

    final source = await _pickSource();
    if (source == null) return;

    (Uint8List, String)? fileData;
    try {
      fileData = await _pickFileData(source);
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not access camera/files. Please try again.')),
      );
      return;
    }

    if (fileData == null) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not read the selected file.')),
      );
      return;
    }

    final (bytes, fileName) = fileData;
    await _uploadDocument(docType, fileName, bytes);
  }

  /// Picks a file via camera capture or the system file picker, returning
  /// its raw bytes and display name — or `null` if the user cancelled or
  /// no usable bytes could be read.
  Future<(Uint8List, String)?> _pickFileData(_UploadSource source) async {
    if (source == _UploadSource.camera) {
      final picked = await ImagePicker().pickImage(source: ImageSource.camera, imageQuality: 90);
      if (picked == null) return null;
      final bytes = await picked.readAsBytes();
      return (bytes, picked.name);
    }

    final result = await FilePicker.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['jpg', 'jpeg', 'png', 'pdf'],
      withData: true,
    );
    if (result == null || result.files.isEmpty) return null;
    final picked = result.files.single;
    final bytes = picked.bytes ?? (picked.path != null ? await File(picked.path!).readAsBytes() : null);
    if (bytes == null) return null;
    return (bytes, picked.name);
  }

  Future<void> _uploadDocument(String docType, String fileName, Uint8List bytes) async {
    final token = AuthProvider.instance.token;
    if (token == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please log in again to upload documents.')),
      );
      return;
    }

    setState(() => _uploading = true);
    try {
      await DocumentApiService.upload(
        token,
        docType: docType,
        fileName: fileName,
        mimeType: _mimeTypeFor(fileName),
        dataBase64: base64Encode(bytes),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('$docType uploaded successfully.')),
      );
      await _loadUploadedDocs();
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not upload document. Please try again.')),
      );
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  Future<File> _materializeFile(UploadedDocument doc) async {
    final bytes = base64Decode(doc.dataBase64);
    if (bytes.isEmpty) {
      throw Exception('This document has no content to save.');
    }
    final dir = await getApplicationDocumentsDirectory();
    final safeName = doc.fileName.trim().isNotEmpty ? doc.fileName.trim() : 'document_${doc.id}';
    final file = File('${dir.path}/$safeName');
    await file.writeAsBytes(bytes, flush: true);

    // Confirm the write actually completed rather than assuming success —
    // a truncated/failed write should surface as an error, not a silent
    // "saved" confirmation.
    final exists = await file.exists();
    final writtenLength = exists ? await file.length() : 0;
    if (!exists || writtenLength != bytes.length) {
      throw Exception('File was not saved correctly.');
    }
    return file;
  }

  Future<void> _viewUploadedDoc(UploadedDocument doc) async {
    try {
      final bytes = base64Decode(doc.dataBase64);
      if (doc.isImage) {
        if (!mounted) return;
        Navigator.of(context).push(
          MaterialPageRoute(builder: (_) => _ImageViewerScreen(title: doc.fileName, bytes: bytes)),
        );
      } else if (doc.isPdf) {
        await Printing.layoutPdf(onLayout: (_) async => bytes);
      } else {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Preview not supported for this file type. Try Download instead.')),
        );
      }
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Could not open this document.')),
      );
    }
  }

  Future<void> _downloadUploadedDoc(UploadedDocument doc) async {
    try {
      await _materializeFile(doc);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Saved "${doc.fileName}" to app storage.')),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not save this document: $e')),
      );
    }
  }

  Future<void> _shareUploadedDoc(UploadedDocument doc) async {
    try {
      final file = await _materializeFile(doc);
      if (doc.isPdf) {
        await Printing.sharePdf(bytes: await file.readAsBytes(), filename: doc.fileName);
      } else {
        await Share.shareXFiles([XFile(file.path)], text: doc.docType);
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not share this document: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final hasAnyFile = _aadhaarFile != null || _panFile != null || _photoFile != null;
    return Scaffold(
      appBar: AppBar(title: const Text('Documents')),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _uploading ? null : _startUpload,
        icon: _uploading
            ? const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
              )
            : const Icon(Icons.upload_file_rounded),
        label: Text(_uploading ? 'Uploading…' : 'Upload Document'),
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 20, 20, 100),
          children: [
            PremiumCard(
              child: Row(
                children: [
                  const Icon(Icons.privacy_tip_rounded, color: AppColors.primary),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      'Your documents are encrypted and used only for KYC verification.',
                      style: theme.textTheme.bodySmall,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),
            _DocumentRow(
              label: 'Aadhaar Card',
              icon: Icons.fingerprint_rounded,
              onStatusChanged: (v) => setState(() => _aadhaar = v),
              onFileCaptured: (f) => setState(() => _aadhaarFile = f),
              onSaveAsPdf: () => _saveAsPdf('Aadhaar Card', _aadhaarFile),
            ),
            const SizedBox(height: 10),
            _DocumentRow(
              label: 'PAN Card',
              icon: Icons.badge_rounded,
              onStatusChanged: (v) => setState(() => _pan = v),
              onFileCaptured: (f) => setState(() => _panFile = f),
              onSaveAsPdf: () => _saveAsPdf('PAN Card', _panFile),
            ),
            const SizedBox(height: 10),
            _DocumentRow(
              label: 'Photograph',
              icon: Icons.face_rounded,
              onStatusChanged: (v) => setState(() => _photo = v),
              onFileCaptured: (f) => setState(() => _photoFile = f),
              onSaveAsPdf: () => _saveAsPdf('Photograph', _photoFile),
            ),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              onPressed: hasAnyFile ? _saveAllAsPdf : null,
              icon: const Icon(Icons.picture_as_pdf_rounded, size: 18),
              label: const Text('Save All as PDF'),
              style: OutlinedButton.styleFrom(minimumSize: const Size.fromHeight(50)),
            ),
            if (_signedDocs.isNotEmpty) ...[
              const SizedBox(height: 24),
              Text('Signed Agreements', style: theme.textTheme.titleMedium),
              const SizedBox(height: 4),
              Text('Loan agreements you\'ve e-signed, with your signature embedded.', style: theme.textTheme.bodySmall),
              const SizedBox(height: 12),
              PremiumCard(
                padding: EdgeInsets.zero,
                child: Column(
                  children: [
                    for (int i = 0; i < _signedDocs.length; i++) ...[
                      ListTile(
                        leading: Container(
                          width: 40,
                          height: 40,
                          decoration: BoxDecoration(
                            color: AppColors.primary.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(AppRadius.sm),
                          ),
                          child: const Icon(Icons.description_rounded, color: AppColors.primary, size: 20),
                        ),
                        title: Text(_signedDocs[i].title, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
                        subtitle: Text('Signed ${DateFormat('d MMM yyyy, h:mm a').format(_signedDocs[i].signedAt)}'),
                        trailing: IconButton(
                          icon: const Icon(Icons.download_rounded, color: AppColors.primary),
                          onPressed: () => _shareSignedDoc(_signedDocs[i]),
                        ),
                      ),
                      if (i != _signedDocs.length - 1) const Divider(height: 1, indent: 16),
                    ],
                  ],
                ),
              ),
            ],
            const SizedBox(height: 24),
            Text('My Uploaded Documents', style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text('Files you\'ve uploaded from your camera or device storage.', style: theme.textTheme.bodySmall),
            const SizedBox(height: 12),
            _UploadedDocumentsSection(
              loading: _loadingUploaded,
              error: _uploadedError,
              docs: _uploadedDocs,
              onRetry: _loadUploadedDocs,
              onView: _viewUploadedDoc,
              onDownload: _downloadUploadedDoc,
              onShare: _shareUploadedDoc,
            ),
            const SizedBox(height: 20),
            PremiumCard(
              child: Row(
                children: [
                  Icon(
                    (_aadhaar && _pan && _photo) ? Icons.verified_rounded : Icons.pending_rounded,
                    color: (_aadhaar && _pan && _photo) ? AppColors.success : AppColors.warning,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      (_aadhaar && _pan && _photo) ? 'Your KYC is verified.' : 'Complete your KYC to unlock faster loan approvals.',
                      style: theme.textTheme.bodyMedium,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _DocumentRow extends StatelessWidget {
  const _DocumentRow({
    required this.label,
    required this.icon,
    required this.onStatusChanged,
    required this.onFileCaptured,
    required this.onSaveAsPdf,
  });

  final String label;
  final IconData icon;
  final ValueChanged<bool> onStatusChanged;
  final ValueChanged<File> onFileCaptured;
  final VoidCallback onSaveAsPdf;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DocumentUploadTile(
          label: label,
          icon: icon,
          onStatusChanged: onStatusChanged,
          onFileCaptured: onFileCaptured,
        ),
        Align(
          alignment: Alignment.centerRight,
          child: TextButton.icon(
            onPressed: onSaveAsPdf,
            icon: const Icon(Icons.picture_as_pdf_rounded, size: 16),
            label: const Text('Save as PDF'),
            style: TextButton.styleFrom(
              foregroundColor: AppColors.primary,
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              visualDensity: VisualDensity.compact,
            ),
          ),
        ),
      ],
    );
  }
}

/// Renders the loading/error/empty/list states for "My Uploaded
/// Documents", fetched from `GET /auth/documents`.
class _UploadedDocumentsSection extends StatelessWidget {
  const _UploadedDocumentsSection({
    required this.loading,
    required this.error,
    required this.docs,
    required this.onRetry,
    required this.onView,
    required this.onDownload,
    required this.onShare,
  });

  final bool loading;
  final String? error;
  final List<UploadedDocument> docs;
  final VoidCallback onRetry;
  final ValueChanged<UploadedDocument> onView;
  final ValueChanged<UploadedDocument> onDownload;
  final ValueChanged<UploadedDocument> onShare;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    if (loading) {
      return const PremiumCard(
        child: Center(
          child: Padding(
            padding: EdgeInsets.symmetric(vertical: 12),
            child: SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2)),
          ),
        ),
      );
    }

    if (error != null) {
      return PremiumCard(
        child: Column(
          children: [
            const Icon(Icons.error_outline_rounded, color: AppColors.error, size: 32),
            const SizedBox(height: 8),
            Text(error!, style: theme.textTheme.bodySmall, textAlign: TextAlign.center),
            const SizedBox(height: 12),
            OutlinedButton(onPressed: onRetry, child: const Text('Try Again')),
          ],
        ),
      );
    }

    if (docs.isEmpty) {
      return PremiumCard(
        child: Row(
          children: [
            const Icon(Icons.folder_off_outlined, color: AppColors.textSecondaryLight),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                'No documents uploaded yet. Tap "Upload Document" to add one.',
                style: theme.textTheme.bodySmall,
              ),
            ),
          ],
        ),
      );
    }

    return PremiumCard(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          for (int i = 0; i < docs.length; i++) ...[
            _UploadedDocumentTile(
              doc: docs[i],
              onView: () => onView(docs[i]),
              onDownload: () => onDownload(docs[i]),
              onShare: () => onShare(docs[i]),
            ),
            if (i != docs.length - 1) const Divider(height: 1, indent: 16),
          ],
        ],
      ),
    );
  }
}

class _UploadedDocumentTile extends StatelessWidget {
  const _UploadedDocumentTile({
    required this.doc,
    required this.onView,
    required this.onDownload,
    required this.onShare,
  });

  final UploadedDocument doc;
  final VoidCallback onView;
  final VoidCallback onDownload;
  final VoidCallback onShare;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return ListTile(
      onTap: onView,
      leading: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: AppColors.primary.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppRadius.sm),
        ),
        child: Icon(
          doc.isImage ? Icons.image_rounded : Icons.picture_as_pdf_rounded,
          color: AppColors.primary,
          size: 20,
        ),
      ),
      title: Text(doc.docType, style: theme.textTheme.titleSmall, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(
        '${doc.fileName} • ${DateFormat('d MMM yyyy').format(doc.createdAt)}',
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      ),
      trailing: PopupMenuButton<String>(
        icon: const Icon(Icons.more_vert_rounded, color: AppColors.textSecondaryLight),
        onSelected: (value) {
          switch (value) {
            case 'view':
              onView();
              break;
            case 'download':
              onDownload();
              break;
            case 'share':
              onShare();
              break;
          }
        },
        itemBuilder: (context) => const [
          PopupMenuItem(value: 'view', child: Text('View')),
          PopupMenuItem(value: 'download', child: Text('Download')),
          PopupMenuItem(value: 'share', child: Text('Share')),
        ],
      ),
    );
  }
}

/// Full-screen pinch-to-zoom viewer for an uploaded image document.
class _ImageViewerScreen extends StatelessWidget {
  const _ImageViewerScreen({required this.title, required this.bytes});

  final String title;
  final Uint8List bytes;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
      ),
      body: Center(
        child: InteractiveViewer(
          minScale: 0.5,
          maxScale: 4,
          child: Image.memory(bytes, fit: BoxFit.contain),
        ),
      ),
    );
  }
}
