#!/usr/bin/php
<?php
/**
 * Example of a task doing some work (sleeping!) and sending regular keep-alives
 * to the manager. The worker is able to detect SIGTERM signals from the manager
 * and should terminate gracefully.
 */


require __DIR__ .'/PoolWorker.php';

/**
 * Implement the setUpTask() and runTask() methods of the worker process
 *
 */
class ExampleTask extends PoolWorker
{
	protected function setUpTask() {
		// init task here
		return true;
	}

	protected function runTask() {
		// $this->terminate is set when catching a SIGTERM
		while (!$this->terminate) {
			echo "Running!\n";
			sleep(1);
			$this->setAlive();
		}
	}
}

/** 
 * Run the task as cli script
 */
try {
	$config = array('keepalive' => array('address'=> 'tcp://localhost:5591'));
	$task = new ExampleTask($config);
	$task->run();
} catch (Exception $e) {
	$msg = $e->getMessage().PHP_EOL.$e->getTraceAsString();
	echo $msg;
	error_log($msg);
	exit(1);
}

exit(0);