import React, { useContext, useEffect, useState } from 'react';
import { Dimmer, Loader, Segment } from 'semantic-ui-react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { API, showError, showSuccess } from '../helpers';
import { UserContext } from '../context/User';

const GitHubOAuth = () => {
  const [searchParams, setSearchParams] = useSearchParams();

  const [userState, userDispatch] = useContext(UserContext);
  const [prompt, setPrompt] = useState('处理中...');
  const [processing, setProcessing] = useState(true);

  let navigate = useNavigate();

  useEffect(() => {
    let isMounted = true;
    let code = searchParams.get('code');
    let state = searchParams.get('state');
    const run = async () => {
      const res = await API.get('/api/oauth/github', { params: { code, state } });
      if (!isMounted) return;
      const { success, message, data } = res.data;
      if (success) {
        if (message === 'bind') {
          showSuccess('绑定成功！');
          navigate('/setting');
        } else {
          userDispatch({ type: 'login', payload: data });
          localStorage.setItem('user', JSON.stringify(data));
          showSuccess('登录成功！');
          navigate('/');
        }
      } else {
        showError(message);
        setPrompt(`操作失败，重定向至登录界面中...`);
        navigate('/setting');
      }
    };
    run();
    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <Segment style={{ minHeight: '300px' }}>
      <Dimmer active inverted>
        <Loader size="large">{prompt}</Loader>
      </Dimmer>
    </Segment>
  );
};

export default GitHubOAuth;
