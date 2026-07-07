import React from 'react';
import { Button, Tooltip } from 'antd';
import { UiIcon } from './UiIcon';

export const IconButton = ({
  children,
  icon,
  iconProps,
  label,
  tooltip,
  ...props
}) => {
  const iconNode = React.isValidElement(icon) ? (
    icon
  ) : (
    <UiIcon icon={icon} {...iconProps} />
  );
  const accessibleLabel =
    label || (typeof tooltip === 'string' ? tooltip : undefined);
  const button = (
    <Button aria-label={accessibleLabel} icon={iconNode} {...props}>
      {children}
    </Button>
  );

  if (!tooltip) {
    return button;
  }

  return <Tooltip title={tooltip}>{button}</Tooltip>;
};

export default IconButton;
