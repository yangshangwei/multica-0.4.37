import { createElement } from "react";
import Svg, { Circle, Ellipse, Line, Path, Polygon, Polyline, Rect } from "react-native-svg";
import { avatarIconNodes, type AvatarIconName } from "@/lib/avatar-icon";

type NodeProps = Record<string, string | number>;
const elements = {
  circle: (props: NodeProps) => <Circle {...props} />,
  ellipse: (props: NodeProps) => <Ellipse {...props} />,
  line: (props: NodeProps) => <Line {...props} />,
  path: (props: NodeProps) => <Path {...props} />,
  polygon: (props: NodeProps) => <Polygon {...props} />,
  polyline: (props: NodeProps) => <Polyline {...props} />,
  rect: (props: NodeProps) => <Rect {...props} />,
};
type IconNode = [keyof typeof elements, Record<string, string | number>];

/** Lucide geometry rendered natively; keeps the web 24px grid and 2px stroke. */
export function AvatarIcon({ name, size, color }: {
  name: AvatarIconName;
  size: number;
  color: string;
}) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      {(avatarIconNodes[name] as IconNode[]).map(([tag, props]) =>
        createElement(elements[tag], props),
      )}
    </Svg>
  );
}
