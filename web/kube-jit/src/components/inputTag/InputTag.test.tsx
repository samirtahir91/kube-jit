import { createRef } from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import InputTag from './InputTag';

const regexPattern = /^[a-z0-9-]+$/;
const tagError = 'Invalid tag!';
const placeholder = 'Add tag...';

describe('InputTag', () => {
  it('renders input with placeholder', () => {
    render(
      <InputTag
        onTagsChange={() => {}}
        setTagError={() => {}}
        regexPattern={regexPattern}
        tagError={tagError}
        id="test-input"
        placeholder={placeholder}
      />
    );
    expect(screen.getByPlaceholderText(placeholder)).toBeInTheDocument();
  });

  it('calls onTagsChange and adds valid tag', () => {
    const onTagsChange = vi.fn();
    const setTagError = vi.fn();
    render(
      <InputTag
        onTagsChange={onTagsChange}
        setTagError={setTagError}
        regexPattern={regexPattern}
        tagError={tagError}
        id="test-input"
        placeholder={placeholder}
      />
    );
    const input = screen.getByPlaceholderText(placeholder);

    fireEvent.change(input, { target: { value: 'valid-tag' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

    // Should call onTagsChange with the new tag
    expect(onTagsChange).toHaveBeenCalledWith(
      expect.arrayContaining([
        expect.objectContaining({ text: 'valid-tag' }),
      ])
    );
    expect(setTagError).toHaveBeenCalledWith('');
  });

  it('shows error for invalid tag', () => {
    const onTagsChange = vi.fn();
    const setTagError = vi.fn();
    render(
      <InputTag
        onTagsChange={onTagsChange}
        setTagError={setTagError}
        regexPattern={regexPattern}
        tagError={tagError}
        id="test-input"
        placeholder={placeholder}
      />
    );
    const input = screen.getByPlaceholderText(placeholder);

    fireEvent.change(input, { target: { value: 'Invalid!' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

    expect(setTagError).toHaveBeenCalledWith(tagError);
  });

  it('can reset tags via ref', () => {
    const onTagsChange = vi.fn();
    const setTagError = vi.fn();
    const ref = createRef<any>();
    render(
      <InputTag
        ref={ref}
        onTagsChange={onTagsChange}
        setTagError={setTagError}
        regexPattern={regexPattern}
        tagError={tagError}
        id="test-input"
        placeholder={placeholder}
      />
    );
    // Add a tag
    const input = screen.getByPlaceholderText(placeholder);
    fireEvent.change(input, { target: { value: 'valid-tag' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter' });

    // Reset tags
    ref.current.resetTags();
    expect(onTagsChange).toHaveBeenCalledWith([]);
  });
});