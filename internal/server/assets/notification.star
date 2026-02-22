load("render.star", "render")
load("encoding/base64.star", "base64")

TITLE_FONT = "tb-8"
SUBTITLE_FONT = "tom-thumb"
ICON_SIZE = 12
ICON_GAP = 1
PADDING = 1
DISPLAY_WIDTH = 64
DISPLAY_HEIGHT = 32

LEVEL_COLORS = {
    "info":    {"title": "#ffffff", "subtitle": "#cccccc", "bg": "#000000"},
    "warning": {"title": "#ffcc00", "subtitle": "#bfa230", "bg": "#1a1500"},
    "alert":   {"title": "#ff3333", "subtitle": "#cc6666", "bg": "#1a0000"},
}

def main(config):
    title = config.str("title", "")
    subtitle = config.str("subtitle", "")
    subtitle2 = config.str("subtitle2", "")
    icon = config.str("icon", "")
    level = config.str("level", "info")

    colors = LEVEL_COLORS.get(level, LEVEL_COLORS["info"])
    title_color = colors["title"]
    subtitle_color = colors["subtitle"]
    bg_color = colors["bg"]

    has_icon = icon != ""
    text_width = DISPLAY_WIDTH - ICON_SIZE - ICON_GAP - (PADDING * 2) if has_icon else DISPLAY_WIDTH - (PADDING * 2)
    text_height = DISPLAY_HEIGHT - (PADDING * 2)

    lines = []
    if title != "":
        lines.append(render.Marquee(
            width = text_width,
            child = render.Text(content = title, font = TITLE_FONT, color = title_color),
        ))
    if subtitle != "":
        lines.append(render.Marquee(
            width = text_width,
            child = render.Text(content = subtitle, font = SUBTITLE_FONT, color = subtitle_color),
        ))
    if subtitle2 != "":
        lines.append(render.Marquee(
            width = text_width,
            child = render.Text(content = subtitle2, font = SUBTITLE_FONT, color = subtitle_color),
        ))

    if len(lines) == 0:
        return []

    if len(lines) > 1:
        spaced = []
        for i in range(len(lines)):
            spaced.append(lines[i])
            if i < len(lines) - 1:
                spaced.append(render.Box(width = 1, height = 1))
        lines = spaced

    text_column = render.Column(
        children = lines,
        expanded = True,
        main_align = "center",
        cross_align = "start",
    )

    text_content = render.Box(
        width = text_width,
        height = text_height,
        child = text_column,
    )

    if has_icon:
        icon_widget = render.Image(src = base64.decode(icon), width = ICON_SIZE, height = ICON_SIZE)
        content = render.Row(
            cross_align = "center",
            children = [
                render.Box(width = ICON_SIZE, height = ICON_SIZE, child = icon_widget),
                render.Box(width = ICON_GAP, height = 1),
                text_content,
            ],
        )
    else:
        content = text_content

    return render.Root(
        child = render.Box(padding = PADDING, color = bg_color, child = content),
    )
