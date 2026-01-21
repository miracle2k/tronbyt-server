load("render.star", "render")
load("encoding/base64.star", "base64")

TITLE_FONT = "tb-8"
SUBTITLE_FONT = "tom-thumb"
TITLE_COLOR = "#ffffff"
SUBTITLE_COLOR = "#cccccc"
ICON_SIZE = 12
ICON_GAP = 1
PADDING = 1

def main(config):
    title = config.str("title", "")
    subtitle = config.str("subtitle", "")
    subtitle2 = config.str("subtitle2", "")
    icon = config.str("icon", "")

    lines = []
    if title != "":
        lines.append(render.Text(content=title, font=TITLE_FONT, color=TITLE_COLOR))
    if subtitle != "":
        lines.append(render.Text(content=subtitle, font=SUBTITLE_FONT, color=SUBTITLE_COLOR))
    if subtitle2 != "":
        lines.append(render.Text(content=subtitle2, font=SUBTITLE_FONT, color=SUBTITLE_COLOR))

    if len(lines) == 0:
        return []

    if len(lines) > 1:
        spaced = []
        for i in range(len(lines)):
            spaced.append(lines[i])
            if i < len(lines) - 1:
                spaced.append(render.Box(width=1, height=1))
        lines = spaced

    text_column = render.Column(
        children = lines,
        main_align = "start",
        cross_align = "start",
    )

    if icon != "":
        icon_widget = render.Image(src = base64.decode(icon), width = ICON_SIZE, height = ICON_SIZE)
        content = render.Row(
            cross_align = "center",
            children = [
                render.Box(width = ICON_SIZE, height = ICON_SIZE, child = icon_widget),
                render.Box(width = ICON_GAP, height = 1),
                render.Box(
                    width = 64 - ICON_SIZE - ICON_GAP - (PADDING * 2),
                    height = 32 - (PADDING * 2),
                    child = text_column,
                ),
            ],
        )
    else:
        content = text_column

    return render.Root(
        child = render.Box(padding = PADDING, child = content),
    )
