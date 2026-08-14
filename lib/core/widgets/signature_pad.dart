import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

/// A real finger-drawn signature capture surface — not a placeholder.
/// Captures the drawn strokes and renders them to a transparent PNG via
/// [capture], used to embed the signature into the generated PDF.
///
/// Drawing only happens in a full-screen editor (opened by tapping the
/// inline preview) — the previous inline-drawing surface sat inside a
/// scrollable (ListView/SingleChildScrollView) on every screen that used
/// it, and even with raw-pointer capture via [Listener], the small inline
/// box made it easy to run a stroke off its edge mid-signature. A
/// full-screen canvas gives the whole screen to draw on and removes the
/// surrounding scrollable entirely.
class SignaturePad extends StatelessWidget {
  const SignaturePad({super.key, required this.controller});

  final SignaturePadController controller;

  Future<void> _openFullscreen(BuildContext context) async {
    final snapshot = controller.strokes.map((s) => List<Offset>.from(s)).toList();
    await Navigator.of(context).push(
      MaterialPageRoute(
        fullscreenDialog: true,
        builder: (_) => _SignatureFullscreenPage(controller: controller, snapshot: snapshot),
      ),
    );
  }

  /// One-off signature capture with no persistent inline preview — pushes
  /// the same full-screen editor directly and returns the signed PNG
  /// bytes (or null if the user closed it without signing). Used for
  /// witness signatures, which don't need a standing form field.
  static Future<Uint8List?> captureFullscreen(BuildContext context) async {
    final controller = SignaturePadController();
    await Navigator.of(context).push(
      MaterialPageRoute(
        fullscreenDialog: true,
        builder: (_) => _SignatureFullscreenPage(controller: controller, snapshot: const []),
      ),
    );
    final bytes = await controller.capture();
    controller.dispose();
    return bytes;
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => _openFullscreen(context),
      child: AnimatedBuilder(
        animation: controller,
        builder: (context, _) => _SignaturePreview(controller: controller),
      ),
    );
  }
}

class _SignaturePreview extends StatelessWidget {
  const _SignaturePreview({required this.controller});

  final SignaturePadController controller;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isEmpty = controller.isEmpty;
    return Container(
      height: 180,
      width: double.infinity,
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.borderLight, width: 1.4),
        borderRadius: BorderRadius.circular(AppRadius.md),
        color: isDark ? AppColors.surfaceDark : Colors.white,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppRadius.md),
        child: Stack(
          alignment: Alignment.center,
          children: [
            if (!isEmpty && controller.canvasSize != null)
              FittedBox(
                child: SizedBox(
                  width: controller.canvasSize!.width,
                  height: controller.canvasSize!.height,
                  child: CustomPaint(painter: _SignaturePainter(controller.strokes)),
                ),
              ),
            if (isEmpty)
              Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.draw_outlined, color: AppColors.textSecondaryLight.withValues(alpha: 0.6), size: 28),
                  const SizedBox(height: 6),
                  Text(
                    'Tap to sign',
                    style: TextStyle(color: AppColors.textSecondaryLight.withValues(alpha: 0.8), fontSize: 13),
                  ),
                ],
              ),
            if (!isEmpty)
              Positioned(
                right: 8,
                bottom: 8,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: AppColors.primary.withValues(alpha: 0.9),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.edit_outlined, color: Colors.white, size: 12),
                      SizedBox(width: 4),
                      Text('Tap to edit', style: TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w600)),
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

class SignaturePadController extends ChangeNotifier {
  final List<List<Offset>> strokes = [];
  List<Offset>? _current;
  final GlobalKey repaintKey = GlobalKey();

  Uint8List? _cachedPng;
  Size? canvasSize;

  bool get isEmpty => strokes.isEmpty;

  void startStroke(Offset point) {
    _cachedPng = null;
    _current = [point];
    strokes.add(_current!);
    notifyListeners();
  }

  void extendStroke(Offset point) {
    _current?.add(point);
    notifyListeners();
  }

  void endStroke() {
    _current = null;
  }

  void clear() {
    strokes.clear();
    _current = null;
    _cachedPng = null;
    canvasSize = null;
    notifyListeners();
  }

  /// Restores strokes to a prior snapshot (used when the full-screen editor
  /// is dismissed without tapping "Done") without exposing [notifyListeners]
  /// itself outside this class.
  void restore(List<List<Offset>> snapshot) {
    strokes
      ..clear()
      ..addAll(snapshot);
    _cachedPng = null;
    notifyListeners();
  }

  /// Renders the current strokes to a transparent PNG and caches the
  /// result, using the RepaintBoundary that's live in the tree right now
  /// (the full-screen editor calls this itself before popping, since by
  /// the time [capture] is called from a submit button elsewhere, that
  /// boundary is long gone).
  Future<void> captureAndCache() async {
    if (isEmpty) return;
    final boundary = repaintKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
    if (boundary == null) return;
    canvasSize = boundary.size;
    final image = await boundary.toImage(pixelRatio: 3);
    final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
    _cachedPng = byteData?.buffer.asUint8List();
  }

  Future<Uint8List?> capture() async {
    if (isEmpty) return null;
    if (_cachedPng != null) return _cachedPng;
    await captureAndCache();
    return _cachedPng;
  }
}

class _SignatureFullscreenPage extends StatefulWidget {
  const _SignatureFullscreenPage({required this.controller, required this.snapshot});

  final SignaturePadController controller;
  final List<List<Offset>> snapshot;

  @override
  State<_SignatureFullscreenPage> createState() => _SignatureFullscreenPageState();
}

class _SignatureFullscreenPageState extends State<_SignatureFullscreenPage> {
  void _onChange() => setState(() {});

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_onChange);
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onChange);
    super.dispose();
  }

  void _restoreAndClose() {
    widget.controller.restore(widget.snapshot);
    Navigator.of(context).pop();
  }

  Future<void> _done() async {
    await widget.controller.captureAndCache();
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final controller = widget.controller;
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        backgroundColor: Colors.white,
        foregroundColor: AppColors.textPrimaryLight,
        elevation: 0,
        leading: IconButton(icon: const Icon(Icons.close), onPressed: _restoreAndClose),
        title: const Text('Sign here'),
        actions: [
          TextButton(
            onPressed: controller.isEmpty ? null : () => setState(controller.clear),
            child: const Text('Clear'),
          ),
          TextButton(
            onPressed: controller.isEmpty ? null : _done,
            child: const Text('Done', style: TextStyle(fontWeight: FontWeight.w700)),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            children: [
              Expanded(
                child: Container(
                  width: double.infinity,
                  decoration: BoxDecoration(
                    border: Border.all(color: AppColors.borderLight, width: 1.4),
                    borderRadius: BorderRadius.circular(AppRadius.md),
                  ),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(AppRadius.md),
                    child: RepaintBoundary(
                      key: controller.repaintKey,
                      child: Listener(
                        onPointerDown: (d) => controller.startStroke(d.localPosition),
                        onPointerMove: (d) => controller.extendStroke(d.localPosition),
                        onPointerUp: (_) => controller.endStroke(),
                        onPointerCancel: (_) => controller.endStroke(),
                        child: SizedBox.expand(
                          child: CustomPaint(painter: _SignaturePainter(controller.strokes)),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              const Text(
                'Draw your signature with your finger, then tap Done',
                style: TextStyle(color: AppColors.textSecondaryLight, fontSize: 12.5),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SignaturePainter extends CustomPainter {
  _SignaturePainter(this.strokes);

  final List<List<Offset>> strokes;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = AppColors.textPrimaryLight
      ..strokeWidth = 2.6
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    for (final stroke in strokes) {
      for (int i = 0; i < stroke.length - 1; i++) {
        canvas.drawLine(stroke[i], stroke[i + 1], paint);
      }
    }
  }

  @override
  bool shouldRepaint(covariant _SignaturePainter oldDelegate) => true;
}
