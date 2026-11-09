#!/bin/sh
# tools/crop.sh name x y w h: the region of the web's and the app's
# screenshots, the web's above, for comparing details at full size.
set -e
n=$1; g="${4}x${5}+${2}+${3}"
convert "compare/web/$n.png" -crop "$g" +repage /tmp/crop-web.png
convert "compare/app/$n.png" -crop "$g" +repage /tmp/crop-app.png
convert /tmp/crop-web.png /tmp/crop-app.png -background '#f0f' -splice 0x4 -append "/tmp/crop-$n.png"
echo "/tmp/crop-$n.png"
