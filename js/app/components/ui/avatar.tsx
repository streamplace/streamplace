import { Avatar, AvatarProps } from "@tamagui/avatar";
import { Fish } from "@tamagui/lucide-icons";

interface UserAvatarProps extends AvatarProps {
  src?: string | null;
}

export function UserAvatar({ src, ...props }: UserAvatarProps) {
  return (
    <Avatar circular size="$4" {...props}>
      {src ? <Avatar.Image accessibilityLabel="User avatar" src={src} /> : null}
      <Avatar.Fallback backgroundColor="$gray8">
        <Fish />
      </Avatar.Fallback>
    </Avatar>
  );
}

export default UserAvatar;
