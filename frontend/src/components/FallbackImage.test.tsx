import { act, create } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';

import { FallbackImage } from './FallbackImage';

describe('FallbackImage', () => {
  it('uses the bundled logo once when the requested image fails', () => {
    let renderer: ReturnType<typeof create>;

    act(() => {
      renderer = create(<FallbackImage alt="avatar" src="https://example.test/avatar.png" />);
    });

    const image = renderer!.root.findByType('img');
    expect(image.props.src).toBe('https://example.test/avatar.png');

    act(() => image.props.onError());

    const fallback = renderer!.root.findByType('img');
    expect(fallback.props.src).toBe('/logo.png');
    expect(fallback.props.onError).toBeUndefined();
  });
});
