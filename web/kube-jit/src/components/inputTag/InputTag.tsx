import { forwardRef, useImperativeHandle, useState } from 'react';
import { WithContext as ReactTags, SEPARATORS, Tag} from 'react-tag-input';
import "./InputTag.css";
  
  interface InputTagProps {
   onTagsChange: (tags: Tag[]) => void;
   setTagError: (error: string) => void;
   regexPattern: RegExp
   tagError: string
   id: string
   placeholder: string
  }
  

  const InputTag = forwardRef(({ onTagsChange, setTagError, regexPattern, tagError, id, placeholder }: InputTagProps, ref) => {
      
    const [tags, setTags] = useState<Tag[]>([]);
    useImperativeHandle(ref, () => ({
        resetTags() {
          setTags([]);
          onTagsChange([]);
        },
        setTagsFromStrings(tagStrings: string[]) {
          const validTags = tagStrings
            .filter(Boolean)
            .filter(text => regexPattern.test(text))
            .map((text, i) => ({ id: `${i}`, text, className: "" }));

          const invalidTags = tagStrings
            .filter(Boolean)
            .filter(text => !regexPattern.test(text));

          setTags(validTags);
          onTagsChange(validTags);

          if (invalidTags.length > 0) {
            setTagError(
              `${tagError} Invalid: ${invalidTags.join(", ")}`
            );
          } else {
            setTagError('');
          }
        }
    }));

  const validateNamespace = (namespace: string) => {
    return regexPattern.test(namespace);
  };
    
  const handleDelete = (i: number) => {
    const newTags = tags.filter((_, index) => index !== i);
    setTags(newTags);
    onTagsChange(newTags);
  };

  const handleAddition = (tag: Tag) => {
    if (validateNamespace(tag.text)) {
      const newTags = [...tags, tag];
      setTags(newTags);
      onTagsChange(newTags);
      setTagError('')
    } else {
      setTagError(tagError);
    }
  };

  return (
    <div id="tags">
      <ReactTags
        id={id}
        separators={[SEPARATORS.ENTER, SEPARATORS.COMMA, SEPARATORS.SPACE]}
        editable
        tags={tags}
        handleDelete={handleDelete}
        handleAddition={handleAddition}
        inputFieldPosition="inline"
        allowDragDrop={false}
        placeholder={placeholder}
      />
    </div>
  );
});

export default InputTag;
