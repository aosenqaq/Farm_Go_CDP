import { useState, type ComponentPropsWithoutRef } from 'react';

type FallbackImageProps = Omit<ComponentPropsWithoutRef<'img'>, 'onError' | 'src'> & {
  src: string;
};

export function FallbackImage({ src, ...props }: FallbackImageProps) {
  const [failed, setFailed] = useState(false);

  return <img {...props} src={failed ? '/logo.png' : src} onError={failed ? undefined : () => setFailed(true)} />;
}
