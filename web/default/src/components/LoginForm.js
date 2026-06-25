import React, { useContext, useEffect, useState } from 'react';
import {
  Button,
  Divider,
  Form,
  Grid,
  Header,
  Image,
  Message,
  Modal,
  Segment,
  Card,
} from 'semantic-ui-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { API, getLogo, showError, showSuccess, showWarning } from '../helpers';
import { onGitHubOAuthClicked, onLarkOAuthClicked } from './oauthUtils';
import larkIcon from '../images/lark.svg';

const LoginForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
    wechat_verification_code: '',
  });
  const [searchParams, setSearchParams] = useSearchParams();
  const [submitted, setSubmitted] = useState(false);
  const { username, password } = inputs;
  const [userState, userDispatch] = useContext(UserContext);
  let navigate = useNavigate();
  const [status, setStatus] = useState({});
  const logo = getLogo();

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('messages.error.login_expired'));
    }
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      setStatus(status);
    }
  }, []);

  const [showWeChatLoginModal, setShowWeChatLoginModal] = useState(false);

  const onWeChatLoginClicked = () => {
    setShowWeChatLoginModal(true);
  };

  const onSubmitWeChatVerificationCode = async () => {
    const res = await API.get(
      `/api/oauth/wechat?code=${inputs.wechat_verification_code}`
    );
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
      localStorage.setItem('user', JSON.stringify(data));
      navigate('/');
      showSuccess(t('messages.success.login'));
      setShowWeChatLoginModal(false);
    } else {
      showError(message);
    }
  };

  function handleChange(e) {
    const { name, value } = e.target;
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }

  async function handleSubmit(e) {
    setSubmitted(true);
    if (username && password) {
      const res = await API.post(`/api/user/login`, {
        username,
        password,
      });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        if (username === 'root' && password === '123456') {
          navigate('/user/edit');
          showSuccess(t('messages.success.login'));
          showWarning(t('messages.error.root_password'));
        } else {
          navigate('/token');
          showSuccess(t('messages.success.login'));
        }
      } else {
        showError(message);
      }
    }
  }

  return (
    <Grid textAlign='center' style={{ marginTop: '48px', minHeight: '60vh', alignContent: 'center' }}>
      <Grid.Column style={{ maxWidth: 420 }}>
        <Card
          fluid
          style={{
            boxShadow: '0 1px 3px rgba(0,0,0,0.08), 0 4px 16px rgba(0,0,0,0.06)',
            border: '1px solid #E5E7EB',
            borderRadius: '12px',
            padding: '8px 0',
          }}
        >
          <Card.Content>
            <Card.Header>
              <Header
                as='h2'
                textAlign='center'
                style={{ marginBottom: '1.5em', fontWeight: '600', letterSpacing: '-0.3px' }}
              >
                <Image src={logo} style={{ marginBottom: '10px' }} />
                <Header.Content>{t('auth.login.title')}</Header.Content>
              </Header>
            </Card.Header>
            <Form size='large'>
              <Form.Input
                fluid
                icon='user'
                iconPosition='left'
                placeholder={t('auth.login.username')}
                name='username'
                value={username}
                onChange={handleChange}
                style={{ marginBottom: '1em' }}
              />
              <Form.Input
                fluid
                icon='lock'
                iconPosition='left'
                placeholder={t('auth.login.password')}
                name='password'
                type='password'
                value={password}
                onChange={handleChange}
                style={{ marginBottom: '1.5em' }}
              />
              <Button
                fluid
                size='large'
                style={{
                  background: 'var(--fb-primary, #3b82f6)',
                  color: 'white',
                  marginBottom: '1.5em',
                }}
                onClick={handleSubmit}
              >
                {t('auth.login.button')}
              </Button>
            </Form>

            <Divider />
            <Message style={{ background: 'transparent', boxShadow: 'none' }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  fontSize: '0.9em',
                  color: '#374151',
                }}
              >
                <div>
                  {t('auth.login.forgot_password')}
                  <Link
                    to='/reset'
                    style={{ color: 'var(--fb-primary, #3b82f6)', marginLeft: '2px' }}
                  >
                    {t('auth.login.reset_password')}
                  </Link>
                </div>
                <div>
                  {t('auth.login.no_account')}
                  <Link
                    to='/register'
                    style={{ color: 'var(--fb-primary, #3b82f6)', marginLeft: '2px' }}
                  >
                    {t('auth.login.register')}
                  </Link>
                </div>
              </div>
            </Message>

            {(status.github_oauth ||
              status.wechat_login ||
              status.lark_client_id) && (
              <>
                <Divider
                  horizontal
                  style={{ color: '#374151', fontSize: '0.9em' }}
                >
                  {t('auth.login.other_methods')}
                </Divider>
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'center',
                    gap: '1em',
                    marginTop: '1em',
                  }}
                >
                  {status.github_oauth && (
                    <Button
                      circular
                      color='black'
                      icon='github'
                      onClick={() =>
                        onGitHubOAuthClicked(status.github_client_id)
                      }
                    />
                  )}
                  {status.wechat_login && (
                    <Button
                      circular
                      color='green'
                      icon='wechat'
                      onClick={onWeChatLoginClicked}
                    />
                  )}
                  {status.lark_client_id && (
                    <div
                      style={{
                        width: '36px',
                        height: '36px',
                        borderRadius: '50%',
                        display: 'flex',
                        cursor: 'pointer',
                      }}
                      onClick={() => onLarkOAuthClicked(status.lark_client_id)}
                    >
                      <Image
                        src={larkIcon}
                        avatar
                        style={{
                          width: '36px',
                          height: '36px',
                          cursor: 'pointer',
                          margin: 'auto',
                        }}
                      />
                    </div>
                  )}
                </div>
              </>
            )}
          </Card.Content>
        </Card>
        <Modal
          onClose={() => setShowWeChatLoginModal(false)}
          onOpen={() => setShowWeChatLoginModal(true)}
          open={showWeChatLoginModal}
          size={'mini'}
        >
          <Modal.Content>
            <Modal.Description>
              <Image src={status.wechat_qrcode} fluid />
              <div style={{ textAlign: 'center' }}>
                <p>{t('auth.login.wechat.scan_tip')}</p>
              </div>
              <Form size='large'>
                <Form.Input
                  fluid
                  placeholder={t('auth.login.wechat.code_placeholder')}
                  name='wechat_verification_code'
                  value={inputs.wechat_verification_code}
                  onChange={handleChange}
                />
                <Button
                  fluid
                  size='large'
                  style={{
                    background: 'var(--fb-primary, #3b82f6)',
                    color: 'white',
                    marginBottom: '1.5em',
                  }}
                  onClick={onSubmitWeChatVerificationCode}
                >
                  {t('auth.login.button')}
                </Button>
              </Form>
            </Modal.Description>
          </Modal.Content>
        </Modal>
      </Grid.Column>
    </Grid>
  );
};

export default LoginForm;
