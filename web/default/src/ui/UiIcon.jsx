import React from 'react';

export const UiIcon = ({
  icon: IconComponent,
  size = 18,
  strokeWidth = 1.8,
  ...props
}) => {
  if (!IconComponent) {
    return null;
  }

  return <IconComponent size={size} strokeWidth={strokeWidth} {...props} />;
};

export default UiIcon;
