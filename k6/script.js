
import http from 'k6/http'
import {check, sleep} from 'k6';

export const options = {
    vus:20, //virtual users
    duration: '5s'
}

export default function () {
    const BASE_URL = 'http://localhost:3000';
    const payload = JSON.stringify({
        message: 'my k6 test',
        chat_id: '',
    });
    const params = {
        headers: { 'Content-Type': 'application/json' },
    };
    const initialRes = http.post(`${BASE_URL}/chat`, payload, params);

    const isRateLimited = check(initialRes, { 'rate limit hit': (r) => r.status === 429,});

   if(isRateLimited){
       return
   }
    const isAccepted = check(initialRes, {
        'job accepted': (r) => r.status === 202,
        'has job_id': (r) => r.json().id !== undefined,
        'has status url':(r) => r.json().status_url !==undefined
    });

    if (isAccepted) {
        const jobId = initialRes.json().id;
        const statusUrl = initialRes.json().status_url
        console.log(statusUrl)
        console.log(jobId)
        let jobCompleted = false;
        let retries = 10;

        while (!jobCompleted && retries > 0) {
            sleep(2);
            const statusReq = JSON.stringify({ job_id: jobId });
            let tempurl = `${BASE_URL}/${statusUrl}`
            console.log(tempurl)
            const pollRes = http.get(tempurl, statusReq, params);


            const currentHttpStatusCode = pollRes.status

            check(pollRes,{
                'status - rate limit':(r)=> r.status === 429
            })

            if(currentHttpStatusCode === 429){
                return;
            }

            check(pollRes, {
                'status fetch 200': (r) => r.status === 200,
            });

            const body = pollRes.json();
            const currentStatus = body.result.status;

            //even if its queued or running status we keep polling
            if (currentStatus === 'COMPLETE') {
                jobCompleted = true;

                check(pollRes, {
                    'has chat_id returned': (r) => r.json().chat_id.length > 0,
                });
                if (!body.error){
                    check(pollRes, {
                        'has answer text': (r) => r.json().result?.rag_response?.answer.length > 0,
                    })
                    console.log(`Job ${jobId} finished. Answer: ${body.result.rag_response.answer.substring(0, 20)}...`);
                }else {
                    check(pollRes, {
                        'has error' : (r) => body.error.code,
                    })
                }


            } else if (body.error) {
                console.error(`Job ${jobId} failed with code: ${body.error.code}`);
                break;
            }
            retries--;
        }
    }
    sleep(1); // Wait 1 second between iterations
}
