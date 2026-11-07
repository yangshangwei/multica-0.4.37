/** Mobile counterpart of the shared web ProjectIcon, preserving its square footprint. */
import { View } from "react-native";
import { useColorScheme } from "nativewind";
import { AvatarIcon } from "./avatar-icon";
import { avatarIconColor, resolveProjectIcon } from "@/lib/avatar-icon";

export type ProjectIconSize = "sm" | "md" | "lg";

const SIZE: Record<ProjectIconSize, { box: number; font: number }> = {
  sm: { box: 18, font: 14 },
  md: { box: 22, font: 16 },
  lg: { box: 28, font: 22 },
};

interface Props {
  icon?: string | null;
  size?: ProjectIconSize;
}

export function ProjectIcon({ icon, size = "sm" }: Props) {
  const { box, font } = SIZE[size];
  const { colorScheme } = useColorScheme();
  const name = resolveProjectIcon(icon);
  return (
    <View
      style={{
        width: box,
        height: box,
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <AvatarIcon name={name} size={font} color={avatarIconColor(name, colorScheme === "dark")} />
    </View>
  );
}
