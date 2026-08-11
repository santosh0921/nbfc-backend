import 'dart:typed_data';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import '../theme/app_colors.dart';
import '../theme/app_radius.dart';

/// A real finger-drawn signature capture surface — not a placeholder.
/// Captures the drawn strokes and renders them to a transparent PNG via
/// [capture], used to embed the signature into the generated PDF.
class SignaturePad extends StatefulWidget {
  const SignaturePad({super.key, required this.controller});

  final SignaturePadController controller;

  @override
  State<SignaturePad> createState() => _SignaturePadState();
}

class SignaturePadController extends ChangeNotifier {
  final List<List<Offset>> strokes = [];
  List<Offset>? _current;
  final GlobalKey repaintKey = GlobalKey();

  bool get isEmpty => strokes.isEmpty;

  void startStroke(Offset point) {
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
    notifyListeners();
  }

  Future<Uint8List?> capture() async {
    if (isEmpty) return null;
    final boundary = repaintKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
    if (boundary == null) return null;
    final image = await boundary.toImage(pixelRatio: 3);
    final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
    return byteData?.buffer.asUint8List();
  }
}

class _SignaturePadState extends State<SignaturePad> {
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

  void _onChange() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.borderLight, width: 1.4),
        borderRadius: BorderRadius.circular(AppRadius.md),
        color: isDark ? AppColors.surfaceDark : Colors.white,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(AppRadius.md),
        child: RepaintBoundary(
          key: widget.controller.repaintKey,
          // Every screen that hosts this pad puts it inside a scrollable
          // (ListView/SingleChildScrollView) alongside other content —
          // GestureDetector's onPan* callbacks enter Flutter's gesture
          // arena and compete with that ancestor's vertical drag
          // recognizer, so a stroke with any vertical motion could lose
          // the arena and scroll the whole screen mid-signature instead
          // of drawing. Listener's raw pointer events bypass the arena
          // entirely and are captured exclusively by whichever widget's
          // hit-test bounds the pointer went down in, which is what
          // dedicated signature-capture packages do for the same reason.
          child: Listener(
            onPointerDown: (d) => widget.controller.startStroke(d.localPosition),
            onPointerMove: (d) => widget.controller.extendStroke(d.localPosition),
            onPointerUp: (_) => widget.controller.endStroke(),
            onPointerCancel: (_) => widget.controller.endStroke(),
            child: SizedBox(
              height: 180,
              width: double.infinity,
              child: CustomPaint(
                painter: _SignaturePainter(widget.controller.strokes),
              ),
            ),
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
