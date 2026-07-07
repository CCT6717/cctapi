import React from 'react';

const joinClassNames = (...values) => values.filter(Boolean).join(' ');

export const PageHeader = ({
  actions,
  children,
  className,
  description,
  kicker,
  title,
  ...props
}) => (
  <header className={joinClassNames('cct-page-header', className)} {...props}>
    <div className='cct-page-header-main'>
      {kicker && <div className='cct-page-header-kicker'>{kicker}</div>}
      <h1 className='cct-page-header-title'>{title}</h1>
      {description && (
        <p className='cct-page-header-description'>{description}</p>
      )}
      {children}
    </div>
    {actions && <div className='cct-page-header-actions'>{actions}</div>}
  </header>
);

export default PageHeader;
