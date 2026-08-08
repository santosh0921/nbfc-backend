import 'dart:convert';
import 'dart:io';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

class SignedDocument {
  const SignedDocument({required this.title, required this.filePath, required this.signedAt});

  final String title;
  final String filePath;
  final DateTime signedAt;

  Map<String, dynamic> toJson() => {'title': title, 'filePath': filePath, 'signedAt': signedAt.toIso8601String()};

  static SignedDocument fromJson(Map<String, dynamic> json) => SignedDocument(
        title: json['title'] as String,
        filePath: json['filePath'] as String,
        signedAt: DateTime.parse(json['signedAt'] as String),
      );
}

/// Persists signed loan agreements (KFS + terms + e-signature, rendered
/// as PDF) to the app's documents directory so they show up under
/// Profile → Documents afterwards, with a real file the user can
/// re-share/download at any time.
class SignedDocumentsStore {
  SignedDocumentsStore._();

  static const _prefsKey = 'signed_documents';

  static Future<SignedDocument> save(String title, List<int> pdfBytes) async {
    final dir = await getApplicationDocumentsDirectory();
    final safeName = title.replaceAll(RegExp(r'[^A-Za-z0-9_]+'), '_');
    final fileName = '${safeName}_${DateTime.now().millisecondsSinceEpoch}.pdf';
    final file = File('${dir.path}/$fileName');
    await file.writeAsBytes(pdfBytes, flush: true);

    final doc = SignedDocument(title: title, filePath: file.path, signedAt: DateTime.now());
    final existing = await list();
    existing.insert(0, doc);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_prefsKey, jsonEncode(existing.map((d) => d.toJson()).toList()));
    return doc;
  }

  static Future<List<SignedDocument>> list() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString(_prefsKey);
    if (raw == null) return [];
    final decoded = jsonDecode(raw) as List<dynamic>;
    return decoded.map((e) => SignedDocument.fromJson(e as Map<String, dynamic>)).toList();
  }
}
