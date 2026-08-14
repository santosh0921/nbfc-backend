import 'dart:math' as math;
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';

/// Premium analog CIBIL gauge: a smooth gradient arc (not flat color
/// blocks) split into Poor/Fair/Good/Excellent zones, tick labels at
/// each boundary, a tapered glass-hub needle with a drop shadow, and an
/// optional slow "live monitoring" sweep dot that travels the arc to
/// signal this is actively tracked, not a static image.
class CreditScoreGauge extends StatefulWidget {
  const CreditScoreGauge({
    super.key,
    required this.score,
    this.minScore = 300,
    this.maxScore = 900,
    this.width = 260,
    this.height = 160,
    this.strokeWidth = 22,
    this.showLabels = false,
    this.live = false,
  });

  final int score;
  final int minScore;
  final int maxScore;
  final double width;
  final double height;
  final double strokeWidth;
  final bool showLabels;
  final bool live;

  @override
  State<CreditScoreGauge> createState() => _CreditScoreGaugeState();
}

class _CreditScoreGaugeState extends State<CreditScoreGauge> with TickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 1100),
  )..forward();
  late final Animation<double> _animation = CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic);

  late final AnimationController _sweepController = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 3200),
  );

  @override
  void initState() {
    super.initState();
    if (widget.live) _sweepController.repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    _sweepController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final fraction = ((widget.score - widget.minScore) / (widget.maxScore - widget.minScore)).clamp(0.0, 1.0);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return AnimatedBuilder(
      animation: Listenable.merge([_animation, _sweepController]),
      builder: (context, _) {
        return SizedBox(
          width: widget.width,
          height: widget.height + (widget.showLabels ? 16 : 0),
          child: CustomPaint(
            painter: _GaugePainter(
              fraction: fraction * _animation.value,
              strokeWidth: widget.strokeWidth,
              showLabels: widget.showLabels,
              isDark: isDark,
              sweepT: widget.live ? _sweepController.value : null,
            ),
          ),
        );
      },
    );
  }
}

class _GaugePainter extends CustomPainter {
  _GaugePainter({
    required this.fraction,
    required this.showLabels,
    required this.isDark,
    this.strokeWidth = 22,
    this.sweepT,
  });

  final double fraction;
  final double strokeWidth;
  final bool showLabels;
  final bool isDark;
  final double? sweepT;

  static const _zoneColors = [
    Color(0xFFE5484D), // Poor
    Color(0xFFFFA23A), // Fair
    Color(0xFFFFD54F), // Good
    Color(0xFF17C964), // Excellent
  ];
  static const _zoneStops = [0.0, 0.25, 0.45, 0.70, 1.0];

  static const startAngle = math.pi; // 180deg, left
  static const sweepAngle = math.pi; // half circle

  @override
  void paint(Canvas canvas, Size size) {
    final center = Offset(size.width / 2, size.height - 8 - (showLabels ? 16 : 0));
    final radius = math.min(size.width / 2, size.height - (showLabels ? 16 : 0)) - strokeWidth / 2 - 2;
    final track = Rect.fromCircle(center: center, radius: radius);

    // Dim full background track for unfilled zones.
    canvas.drawArc(
      track,
      startAngle,
      sweepAngle,
      false,
      Paint()
        ..shader = const SweepGradient(
          startAngle: startAngle,
          endAngle: startAngle + sweepAngle,
          colors: _zoneColors,
          stops: [0.0, 0.30, 0.55, 1.0],
        ).createShader(track)
        ..style = PaintingStyle.stroke
        ..strokeWidth = strokeWidth
        ..strokeCap = StrokeCap.round
        ..color = Colors.white.withValues(alpha: isDark ? 0.10 : 0.14),
    );
    canvas.saveLayer(track.inflate(strokeWidth), Paint());
    canvas.drawArc(
      track,
      startAngle,
      sweepAngle,
      false,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = strokeWidth
        ..strokeCap = StrokeCap.round
        ..color = Colors.white.withValues(alpha: isDark ? 0.14 : 0.22),
    );
    canvas.restore();

    // Filled gradient arc up to the current score.
    if (fraction > 0.001) {
      final filledSweep = sweepAngle * fraction;
      canvas.drawArc(
        track,
        startAngle,
        filledSweep,
        false,
        Paint()
          ..shader = const SweepGradient(
            startAngle: startAngle,
            endAngle: startAngle + sweepAngle,
            colors: _zoneColors,
            stops: [0.0, 0.30, 0.55, 1.0],
          ).createShader(track)
          ..style = PaintingStyle.stroke
          ..strokeWidth = strokeWidth
          ..strokeCap = StrokeCap.round,
      );

      // Soft glow under the tip of the filled arc.
      final tipAngle = startAngle + filledSweep;
      final tipColor = _colorAt(fraction);
      final tip = Offset(center.dx + radius * math.cos(tipAngle), center.dy + radius * math.sin(tipAngle));
      canvas.drawCircle(tip, strokeWidth * 0.62, Paint()..color = tipColor.withValues(alpha: 0.55)..maskFilter = const MaskFilter.blur(BlurStyle.normal, 8));
    }

    // Zone boundary ticks.
    for (final stop in _zoneStops) {
      final angle = startAngle + sweepAngle * stop;
      final inner = Offset(center.dx + (radius - strokeWidth / 2 - 3) * math.cos(angle), center.dy + (radius - strokeWidth / 2 - 3) * math.sin(angle));
      final outer = Offset(center.dx + (radius + strokeWidth / 2 + 3) * math.cos(angle), center.dy + (radius + strokeWidth / 2 + 3) * math.sin(angle));
      canvas.drawLine(inner, outer, Paint()..color = (isDark ? Colors.black : Colors.white).withValues(alpha: 0.9)..strokeWidth = 2);
    }

    // Optional slow "live monitoring" sweep dot along the filled arc.
    if (sweepT != null && fraction > 0.02) {
      final t = sweepT!;
      final pulse = (math.sin(t * 2 * math.pi) + 1) / 2;
      final dotAngle = startAngle + sweepAngle * fraction * (0.15 + 0.85 * pulse);
      final dot = Offset(center.dx + radius * math.cos(dotAngle), center.dy + radius * math.sin(dotAngle));
      canvas.drawCircle(dot, strokeWidth * 0.34, Paint()..color = Colors.white.withValues(alpha: 0.9));
      canvas.drawCircle(dot, strokeWidth * 0.34, Paint()..color = _colorAt(fraction).withValues(alpha: 0.6)..style = PaintingStyle.stroke..strokeWidth = 2);
    }

    // Needle — tapered teardrop shape with a subtle shadow.
    final needleAngle = startAngle + sweepAngle * fraction;
    final needleLength = radius - strokeWidth / 2 - 6;
    final baseWidth = (strokeWidth / 6).clamp(2.0, 4.5);
    final perp = needleAngle + math.pi / 2;
    final baseL = Offset(center.dx + baseWidth * math.cos(perp), center.dy + baseWidth * math.sin(perp));
    final baseR = Offset(center.dx - baseWidth * math.cos(perp), center.dy - baseWidth * math.sin(perp));
    final tipPt = Offset(center.dx + needleLength * math.cos(needleAngle), center.dy + needleLength * math.sin(needleAngle));
    final needlePath = Path()
      ..moveTo(baseL.dx, baseL.dy)
      ..lineTo(tipPt.dx, tipPt.dy)
      ..lineTo(baseR.dx, baseR.dy)
      ..close();

    canvas.drawShadow(needlePath, Colors.black, 3, false);
    canvas.drawPath(
      needlePath,
      Paint()
        ..shader = LinearGradient(colors: [_colorAt(fraction), isDark ? AppColors.textPrimaryDark : AppColors.textPrimaryLight])
            .createShader(Rect.fromPoints(center, tipPt)),
    );

    // Glass hub.
    final hubRadius = (strokeWidth / 2.4).clamp(4.0, 9.0);
    canvas.drawCircle(center, hubRadius + 2, Paint()..color = Colors.black.withValues(alpha: 0.15)..maskFilter = const MaskFilter.blur(BlurStyle.normal, 3));
    canvas.drawCircle(
      center,
      hubRadius,
      Paint()
        ..shader = RadialGradient(
          colors: [Colors.white, (isDark ? AppColors.textPrimaryDark : AppColors.textPrimaryLight).withValues(alpha: 0.9)],
          center: const Alignment(-0.3, -0.3),
        ).createShader(Rect.fromCircle(center: center, radius: hubRadius)),
    );
    canvas.drawCircle(center, hubRadius * 0.42, Paint()..color = _colorAt(fraction));

    if (showLabels) {
      _drawLabel(canvas, '300', Offset(center.dx - radius - strokeWidth / 2, center.dy + 14), isDark, TextAlign.left);
      _drawLabel(canvas, '900', Offset(center.dx + radius + strokeWidth / 2, center.dy + 14), isDark, TextAlign.right);
    }
  }

  void _drawLabel(Canvas canvas, String text, Offset anchor, bool isDark, TextAlign align) {
    final painter = TextPainter(
      text: TextSpan(
        text: text,
        style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: (isDark ? Colors.white : Colors.black).withValues(alpha: 0.4)),
      ),
      textAlign: align,
      textDirection: TextDirection.ltr,
    )..layout();
    final dx = align == TextAlign.left ? anchor.dx : anchor.dx - painter.width;
    painter.paint(canvas, Offset(dx, anchor.dy));
  }

  Color _colorAt(double t) {
    if (t <= 0) return _zoneColors.first;
    if (t >= 1) return _zoneColors.last;
    const stops = [0.0, 0.30, 0.55, 1.0];
    for (int i = 0; i < stops.length - 1; i++) {
      if (t >= stops[i] && t <= stops[i + 1]) {
        final localT = (t - stops[i]) / (stops[i + 1] - stops[i]);
        return Color.lerp(_zoneColors[i], _zoneColors[i + 1], localT)!;
      }
    }
    return _zoneColors.last;
  }

  @override
  bool shouldRepaint(covariant _GaugePainter oldDelegate) =>
      oldDelegate.fraction != fraction || oldDelegate.sweepT != sweepT || oldDelegate.isDark != isDark;
}
